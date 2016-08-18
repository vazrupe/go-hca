go-hca
======
[cri hca audio](http://www.criware.com/en/products/adx2.html) decoder with golang

Installation
------------

    go get github.com/vazrupe/go-hca

Usage
-----

    import (
        ...
        hca "github.com/vazrupe/go-hca"
        ...
    )

    ...
    hca.NewDecoder()
    result, ok := hca.DecodeFromBytes(YOUR_[]byte_DATA)
    if ok {
        ... your code ...
    }
    ...

Lisence
-------
WTFPL 2.0

Reference
---------
[HCA decoder v1.12](https://mega.nz/#!Fh8FwKoB!0xuFdrit3IYcEgQK7QIqFKG3HMQ6rHKxrH3r5DJlJ3M)