package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/webscalesoftwareltd/hypercache/radix"
)

func setupTree(path, name string, sleepTime time.Duration) radix.RadixTree {
	//if path == "" {
	return radix.NewRadixTree()
	//}
	//
	////rbf := radix.NewRBF(path)
	//var tree radix.RadixTree
	////ret := rbf.Load()
	//if ret == nil {
	//	fmt.Println("[WARN]", name, "could not be loaded from disk")
	//	tree = radix.NewRadixTree()
	//} else {
	//	fmt.Println("[LOG]", name, "loaded from disk")
	//	tree = *ret
	//}
	//
	//go func() {
	//	for {
	//		time.Sleep(sleepTime)
	//		if rbf.Write(tree) {
	//			fmt.Println("[LOG]", name, "written to disk")
	//		} else {
	//			_, _ = fmt.Fprintln(os.Stderr, "[ERROR]", name, "could not be written to disk")
	//		}
	//	}
	//}()
	//
	//return tree
}

var (
	trees            []radix.RadixTree
	mutexes          []sync.Mutex
	eventDispatchers []eventDispatcher
	password         []byte
)

func main() {
	dbCountPtr := flag.Uint("db-count", 10, "the number of databases")
	writeDurationPtr := flag.Duration("write-duration", time.Minute*5, "the amount of time between saves - minimum 10 seconds")
	dataPathPtr := flag.String("data-path", "./data", "defines the path where data is stored")
	savesPtr := flag.Bool("saves", true, "defines if the database should be read/saved from disk")
	passwordPtr := flag.String("password", "", "defines the database password")
	hnpBindPtr := flag.String("hnp-bind", "127.0.0.1:6060", "defines the bind for the HyperCache Networking Protocol")
	httpBindPtr := flag.String("http-bind", "127.0.0.1:6061", "defines the bind for the HTTP implementation")
	flag.Parse()

	dbCount := *dbCountPtr
	if dbCount == 0 {
		dbCount = 10
	}
	writeDuration := *writeDurationPtr
	if writeDuration == 0 {
		writeDuration = time.Minute * 5
	}
	dataPath := *dataPathPtr
	saves := *savesPtr
	if !saves {
		dataPath = ""
	}
	password = []byte(*passwordPtr)

	trees = make([]radix.RadixTree, dbCount)
	mutexes = make([]sync.Mutex, dbCount)
	eventDispatchers = make([]eventDispatcher, dbCount)
	err := os.MkdirAll(dataPath, 0o777)
	if err != nil {
		panic(err)
	}
	p, err := filepath.Abs(dataPath)
	if err != nil {
		panic(err)
	}
	for i := range trees {
		fp := filepath.Join(p, strconv.Itoa(i)+".rbf")
		trees[i] = setupTree(fp, "DB "+strconv.Itoa(i), writeDuration)
	}

	go func() {
		fmt.Println("[LOG] HNP handler going to serve on", *hnpBindPtr)
		ln, err := net.Listen("tcp", *hnpBindPtr)
		if err != nil {
			panic(err)
		}
		for {
			conn, err := ln.Accept()
			if err != nil {
				panic(err)
			}
			go spawnHnpHandler(conn)
		}
	}()

	fmt.Println("[LOG] HTTP handler going to serve on", *httpBindPtr)
	err = http.ListenAndServe(*httpBindPtr, httpHn)
	if err != nil {
		panic(err)
	}
}
