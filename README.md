Pofw: A network port forwarding program
=======================================

This is a program that forwards data among network ports.

It listens on some TCP or UDP ports, then forward any requests to some other TCP or UDP ports.

It is like [socat](http://www.dest-unreach.org/socat/), but handles concurrency.

Building
--------

Download the source code, and the latest [Go](https://golang.org/dl/).

Type:

```
go build
```

Configuring
-----------

Open `pofw.conf`, each line is a forwarding rule, in the form of

```
<form protocol> <from address>  <to protocol> <to_address>
```

`Protocol` can be `tcp`, `tcp4`, `tcp6`, `udp`, `udp4`, `udp6`, `unix`, `unixgram`.

For `tcp`, `tcp4`, `udp` and `udp4`, `address` is in the form of `ip:port`.

For `tcp`, `tcp6`, `udp` and `udp6`, `address` is in the form of `[ip]:port`.

If you leave `ip` as empty, then all IP addresses on the machine will be listened on.

You can convert data from UDP to TCP, then back to UDP. The speed will be slower, but it is useful if UDP packets are blocked by a firewall.

Note that only data originally coming from a UDP port can be eventually converted back to UDP: You cannot convert arbitary TCP data to UDP, or the program will report "packet too large".

Running
-------

Simply type `./pofw`, there it go!

You can also type `./pofw myconf.conf`, where `myconf.conf` is a configuration file other than `pofw.conf`.

License
-------

This program is licensed under GPL version 3. See [COPYING](COPYING) for details.

See Also
--------

Also please check [portpub](http://github.com/m13253/portpub), which publishes a service from localhost onto your server.
