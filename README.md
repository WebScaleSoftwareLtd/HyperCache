# HyperCache

A fast auto-namespacing cache that is powered by a custom radix tree library.

NOTE: This is still a work in progress, a lot of the documentation and libraries does not exist or is very immature.

## Supported Features
HyperCache supports the following:
- Item get/put/delete
- Item prefix fetching/bulk deletion
- Whole tree wiping
- Custom event dispatching
- Built in network mutex support
- Multi-threaded out of the box

The key difference between this cache and something like Redis is how the tree is internally managed. With our radix tree solution, you get the ability to get all of the data with a certain prefix and delete it. This is more powerful than other caching solutions because say you want to purge a user from the cache, instead of having to tediously keep a record of each key related to the user, you can just purge `user:`. Unlike other caches, accessing prefixes has zero cost due to it just following the branches like it regularly would.

The mutex functionality is very powerful too. Whilst Redis has solutions for mutexes, these are generally fairly tedious and involve performing an action and waiting a specified amount of time. Due to our custom networking protocol, replies are next to zero cost and allow us to deliver it using Go's built-in goroutine mechanisms.

## Building

TODO: This will contain more information.

To build HyperCache, you will need Swig installed on your computer. From here, you can follow this command:
```
$ swig -go -cgo -O -c++ radix/radix.i && CGO_CXXFLAGS=-std=c++17 go build .
```
