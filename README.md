# Go Serial library

[![](https://goci.herokuapp.com/project/image/github.com/samofly/serial)](http://goci.me/project/github.com/samofly/serial)

Currently, it only supports Linux and is mostly tested on ARM and x86_64 architectures.
There's no dependency on CGO and it directly calls Linux kernel.

The implementation uses some public-domain headers from [musl-libc](http://www.musl-libc.org), manually converted to Go.

Currently, there's no support for non-standard baud rates, like 250000, but they will likely be implemented, since many devices based on Ardiono will use it to reduce error ratio from jitter.
