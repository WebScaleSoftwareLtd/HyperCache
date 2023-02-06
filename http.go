package main

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"unsafe"

	"github.com/gorilla/mux"
	"github.com/webscalesoftwareltd/hypercache/radix"
)

var httpHn = mux.NewRouter()

var bearer = []byte("Password ")

func throwException(exceptionName, exceptionDescription string, w http.ResponseWriter) {
	w.Header().Set("X-Exception", exceptionName)
	w.WriteHeader(http.StatusBadRequest)
	_, _ = w.Write([]byte(exceptionDescription))
}

func getDb(w http.ResponseWriter, r *http.Request) (*radix.RadixTree, bool) {
	vars := mux.Vars(r)
	value, ok := vars["db"]
	if !ok {
		throwException(
			"DatabaseNotFound",
			"The database value is not present.",
			w)
		return nil, true
	}
	i, err := strconv.Atoi(value)
	if err != nil {
		throwException(
			"DatabaseNotFound",
			"The database number is invalid.",
			w)
		return nil, true
	}
	if i >= len(trees) {
		throwException(
			"DatabaseNotFound",
			"The database index is too large for the number of databases in this application.",
			w)
		return nil, true
	}
	return &trees[i], false
}

func s2b(s string) (b []byte) {
	bh := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	sh := (*reflect.StringHeader)(unsafe.Pointer(&s))
	bh.Data = sh.Data
	bh.Cap = sh.Len
	bh.Len = sh.Len
	return b
}

var (
	trueB  = []byte("true")
	falseB = []byte("false")
)

func b2s(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

func setupApiV1(apiV1 *mux.Router) {
	// Handle key fetching, insertion, and deletion.
	apiV1.HandleFunc("/record/{key}", func(w http.ResponseWriter, r *http.Request) {
		db, ret := getDb(w, r)
		if ret {
			return
		}

		vars := mux.Vars(r)
		key := s2b(vars["key"])
		if r.Method == "GET" {
			value, deallocator := db.Get(key)
			defer func() { go deallocator() }()
			if value == nil {
				throwException(
					"NotFound",
					"The key was not found in the database.",
					w)
			} else {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(value)
			}
			return
		}

		if r.Method == "PUT" {
			defer r.Body.Close()
			body, err := io.ReadAll(r.Body)
			if err != nil {
				return
			}

			res := db.Set(key, body)
			w.WriteHeader(http.StatusOK)
			var b []byte
			if res {
				b = trueB
			} else {
				b = falseB
			}
			_, _ = w.Write(b)
			return
		}

		res := db.DeleteKey(key)
		w.WriteHeader(http.StatusOK)
		var b []byte
		if res {
			b = trueB
		} else {
			b = falseB
		}
		_, _ = w.Write(b)
	}).Methods("GET", "PUT", "DELETE")

	// Handle prefix walking and deletion.
	apiV1.HandleFunc("/prefix/{prefix}", func(w http.ResponseWriter, r *http.Request) {
		db, ret := getDb(w, r)
		if ret {
			return
		}

		vars := mux.Vars(r)
		prefix := s2b(vars["prefix"])
		if r.Method == "DELETE" {
			res := db.DeletePrefix(prefix)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(strconv.FormatUint(res, 10)))
			return
		}

		m := map[string]string{}
		freer := &radix.PendingFreer{}
		db.WalkPrefix(prefix, func(key, value []byte) bool {
			m[string(key)] = string(value)
			return true
		}, freer)
		go freer.FreeAll()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(m)
	}).Methods("GET", "DELETE")

	// Deletes the tree.
	apiV1.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		db, ret := getDb(w, r)
		if ret {
			return
		}

		db.FreeTree()
		w.WriteHeader(http.StatusNoContent)
	}).Methods("DELETE")
}

func init() {
	// Add some middleware to check the authorization header and set the content type.
	httpHn.Use(func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := []byte(r.Header.Get("Authorization"))
			if bytes.HasPrefix(authHeader, bearer) {
				authHeader = authHeader[len(bearer):]
			}
			w.Header().Set("Content-Type", "application/octet-stream")
			if subtle.ConstantTimeCompare(authHeader, password) != 1 {
				throwException(
					"InvalidCredentials",
					"The specified password is invalid.",
					w)
				return
			}
			handler.ServeHTTP(w, r)
		})
	})

	// Add API V1.
	apiV1 := httpHn.PathPrefix("/api/v1/{db:[0-9]+}").Subrouter()
	setupApiV1(apiV1)
}
