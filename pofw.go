/*
    Pofw -- A network port forwarding program
    Copyright (C) 2016 Star Brilliant <m13253@hotmail.com>

    This program is free software: you can redistribute it and/or modify
    it under the terms of the GNU General Public License as published by
    the Free Software Foundation, either version 3 of the License, or
    (at your option) any later version.

    This program is distributed in the hope that it will be useful,
    but WITHOUT ANY WARRANTY; without even the implied warranty of
    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
    GNU General Public License for more details.

    You should have received a copy of the GNU General Public License
    along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/
package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	conf_path := "pofw.conf"
	if len(os.Args) == 2 {
		if os.Args[1] == "--help" {
			printUsage()
			os.Exit(0)
		} else {
			conf_path = os.Args[1]
		}
	} else if len(os.Args) != 1 {
		printUsage()
		os.Exit(1)
	}
	conf_file, err := os.Open(conf_path)
	if err != nil {
		log.Fatalln("cannot open configuration file:", err)
	}
	conf_scanner := bufio.NewScanner(conf_file)
	conf_linecount := 0
	for conf_scanner.Scan() {
		conf_linecount++
		line := strings.SplitN(conf_scanner.Text(), "#", 2)[0]
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		} else if len(fields) != 4 {
			log.Fatalf("line %d: requires four parameters 'from protocol' 'from address' 'to protocol' 'to address'\n", conf_linecount)
		} else if err = startForwarding(fields[0], fields[1], fields[2], fields[3]); err != nil {
			log.Fatalln(err)
		}
	}
	conf_file.Close()
	if err = conf_scanner.Err(); err != nil {
		log.Fatalln("cannot read configuration file:", err)
	}
	<-make(chan bool)
}

func printUsage() {
	fmt.Printf("Usage: %s [CONFIG]\n\n  CONFIG\tConfiguration file [Default: pofw.conf]\n\n", os.Args[0])
}

func startForwarding(from_protocol, from_address, to_protocol, to_address string) error {
	switch from_protocol {
	case "udp", "udp4", "udp6", "ip", "ip4", "ip6", "unixgram":
		return startForwardingPacket(from_protocol, from_address, to_protocol, to_address)
	default:
		return startForwardingStream(from_protocol, from_address, to_protocol, to_address)
	}
}

func startForwardingStream(from_protocol, from_address, to_protocol, to_address string) error {
	listener, err := net.Listen(from_protocol, from_address)
	if err != nil {
		return err
	}
	log.Printf("serving on %s %s\n", listener.Addr().Network(), listener.Addr().String())
	go func() {
		for {
			conn_in, err := listener.Accept()
			if err != nil {
				log.Printf("%s ? <-!-> %s %s <===> %s ? <---> %s %s\n", listener.Addr().Network(), listener.Addr().Network(), listener.Addr().String(), to_protocol, to_protocol, to_address)
				log.Fatalln(err)
			}
			go func() {
				conn_out, err := net.Dial(to_protocol, to_address)
				if err != nil {
					log.Printf("%s %s <---> %s %s <===> %s ? <-!-> %s %s\n", conn_in.RemoteAddr().Network(), conn_in.RemoteAddr().String(), conn_in.LocalAddr().Network(), conn_in.LocalAddr().String(), to_protocol, to_protocol, to_address)
					log.Println(err)
					conn_in.Close()
					return
				}
				log.Printf("%s %s <---> %s %s <===> %s %s <---> %s %s\n", conn_in.RemoteAddr().Network(), conn_in.RemoteAddr().String(), conn_in.LocalAddr().Network(), conn_in.LocalAddr().String(), conn_out.LocalAddr().Network(), conn_out.LocalAddr().String(), conn_out.RemoteAddr().Network(), conn_out.RemoteAddr().String())
				go func() {
					var err error
					var packet_len int
					buffer := make([]byte, 2048)
					if _, conn_out_is_dgram := conn_out.(net.PacketConn); conn_out_is_dgram {
						for {
							_, err = io.ReadFull(conn_in, buffer[:2])
							if err != nil { break }
							packet_len = (int(buffer[0]) << 8) | int(buffer[1])
							if packet_len > 2046 {
								err = &tooLargePacketError {
									Size: packet_len,
								}
								break
							}
							_, err = io.ReadFull(conn_in, buffer[2:2+packet_len])
							if err != nil { break }
							_, err = conn_out.Write(buffer[2:2+packet_len])
							if err != nil { break }
						}
					} else {
						for {
							packet_len, err = conn_in.Read(buffer)
							if err != nil { break }
							_, err = conn_out.Write(buffer[:packet_len])
							if err != nil { break }
						}
					}
					if err == io.EOF {
						log.Printf("%s %s <---> %s %s ==X=> %s %s <---> %s %s\n", conn_in.RemoteAddr().Network(), conn_in.RemoteAddr().String(), conn_in.LocalAddr().Network(), conn_in.LocalAddr().String(), conn_out.LocalAddr().Network(), conn_out.LocalAddr().String(), conn_out.RemoteAddr().Network(), conn_out.RemoteAddr().String())
					} else {
						log.Printf("%s %s <---> %s %s ==!=> %s %s <---> %s %s\n", conn_in.RemoteAddr().Network(), conn_in.RemoteAddr().String(), conn_in.LocalAddr().Network(), conn_in.LocalAddr().String(), conn_out.LocalAddr().Network(), conn_out.LocalAddr().String(), conn_out.RemoteAddr().Network(), conn_out.RemoteAddr().String())
						log.Println(err)
					}
					if conn_in_tcp, ok := conn_in.(*net.TCPConn); ok {
						conn_in_tcp.CloseRead()
					}
					if conn_out_tcp, ok := conn_out.(*net.TCPConn); ok {
						conn_out_tcp.CloseWrite()
					} else {
						conn_out.Close()
					}
				}()
				go func() {
					var err error
					var packet_len int
					buffer := make([]byte, 2048)
					if _, conn_out_is_dgram := conn_out.(net.PacketConn); conn_out_is_dgram {
						for {
							packet_len, err = conn_out.Read(buffer[2:])
							if err != nil { break }
							buffer[0], buffer[1] = byte(packet_len >> 8), byte(packet_len)
							_, err = conn_in.Write(buffer[:2+packet_len])
							if err != nil { break }
						}
					} else {
						for {
							packet_len, err = conn_out.Read(buffer)
							if err != nil { break }
							_, err = conn_in.Write(buffer[:packet_len])
							if err != nil { break }
						}
					}
					if err == io.EOF {
						log.Printf("%s %s <---> %s %s <=X== %s %s <---> %s %s\n", conn_in.RemoteAddr().Network(), conn_in.RemoteAddr().String(), conn_in.LocalAddr().Network(), conn_in.LocalAddr().String(), conn_out.LocalAddr().Network(), conn_out.LocalAddr().String(), conn_out.RemoteAddr().Network(), conn_out.RemoteAddr().String())
					} else {
						log.Printf("%s %s <---> %s %s <=!== %s %s <---> %s %s\n", conn_in.RemoteAddr().Network(), conn_in.RemoteAddr().String(), conn_in.LocalAddr().Network(), conn_in.LocalAddr().String(), conn_out.LocalAddr().Network(), conn_out.LocalAddr().String(), conn_out.RemoteAddr().Network(), conn_out.RemoteAddr().String())
						log.Println(err)
					}
					if conn_out_tcp, ok := conn_out.(*net.TCPConn); ok {
						conn_out_tcp.CloseRead()
					}
					if conn_in_tcp, ok := conn_in.(*net.TCPConn); ok {
						conn_in_tcp.CloseWrite()
					} else {
						conn_in.Close()
					}
				}()
			}()
		}
	}()
	return nil
}

func startForwardingPacket(from_protocol, from_address, to_protocol, to_address string) error {
	conn_in, err := net.ListenPacket(from_protocol, from_address)
	if err != nil {
		return err
	}
	log.Printf("serving on %s %s\n", conn_in.LocalAddr().Network(), conn_in.LocalAddr().String())
	go func() {
		type pipe_cache struct {
			Pipe    *io.PipeWriter
			Ready   *uintptr
			TTL     time.Time
		}
		type hashable_addr struct {
			Network string
			String  string
		}
		pipes := make(map[hashable_addr]pipe_cache)
		pipes_lock := new(sync.RWMutex)
		buffer := make([]byte, 2048)
		for {
			packet_len, addr_in, err := conn_in.ReadFrom(buffer)
			if err != nil {
				if err_net, ok := err.(net.Error); ok {
					if err_net.Temporary() {
						continue
					}
				}
				log.Printf("%s ? <-!-> %s %s <===> %s ? <---> %s %s\n", conn_in.LocalAddr().Network(), conn_in.LocalAddr().Network(), conn_in.LocalAddr().String(), to_protocol, to_protocol, to_address)
				log.Fatalln(err)
			}
			pipes_lock.RLock()
			if pipe_out, ok := pipes[hashable_addr {
				Network: addr_in.Network(),
				String: addr_in.String(),
			}]; ok {
				pipes_lock.RUnlock()
				pipe_out.TTL = time.Now().Add(180 * time.Second)
				if atomic.LoadUintptr(pipe_out.Ready) != 0 {
					pipe_out.Pipe.Write(buffer[:packet_len])
				}
			} else {
				pipes_lock.RUnlock()
				first_packet := make([]byte, packet_len)
				copy(first_packet, buffer)
				go func(addr_in net.Addr, first_packet []byte) {
					conn_out, err := net.Dial(to_protocol, to_address)
					if err != nil {
						log.Printf("%s %s <---> %s %s <===> %s ? <-!-> %s %s\n", addr_in.Network(), addr_in.String(), conn_in.LocalAddr().Network(), conn_in.LocalAddr().String(), to_protocol, to_protocol, to_address)
						log.Println(err)
						return
					}
					log.Printf("%s %s <---> %s %s <===> %s %s <---> %s %s\n", addr_in.Network(), addr_in.String(), conn_in.LocalAddr().Network(), conn_in.LocalAddr().String(), conn_out.LocalAddr().Network(), conn_out.LocalAddr().String(), conn_out.RemoteAddr().Network(), conn_out.RemoteAddr().String())
					pipe_in, pipe_out := io.Pipe()
					ready := new(uintptr)
					pipe := pipe_cache {
						Pipe: pipe_out,
						Ready: ready,
						TTL: time.Now().Add(180 * time.Second),
					}
					pipes_lock.Lock()
					pipes[hashable_addr {
						Network: addr_in.Network(),
						String: addr_in.String(),
					}] = pipe
					pipes_lock.Unlock()
					go func() {
						var err error
						var packet_len int
						buffer := make([]byte, 2048)
						if _, conn_out_is_dgram := conn_out.(net.PacketConn); conn_out_is_dgram {
							for {
								atomic.StoreUintptr(ready, 1)
								packet_len, err = pipe_in.Read(buffer)
								atomic.StoreUintptr(ready, 0)
								if err != nil { break }
								_, err = conn_out.Write(buffer[:packet_len])
								if err != nil { break }
							}
						} else {
							for {
								atomic.StoreUintptr(ready, 1)
								packet_len, err = pipe_in.Read(buffer[2:])
								atomic.StoreUintptr(ready, 0)
								if err != nil { break }
								buffer[0], buffer[1] = byte(packet_len >> 8), byte(packet_len)
								_, err = conn_out.Write(buffer[:2+packet_len])
								if err != nil { break }
							}
						}
						if err == io.EOF {
							log.Printf("%s %s <---> %s %s ==X=> %s %s <---> %s %s\n", addr_in.Network(), addr_in.String(), conn_in.LocalAddr().Network(), conn_in.LocalAddr().String(), conn_out.LocalAddr().Network(), conn_out.LocalAddr().String(), conn_out.RemoteAddr().Network(), conn_out.RemoteAddr().String())
						} else {
							log.Printf("%s %s <---> %s %s ==!=> %s %s <---> %s %s\n", addr_in.Network(), addr_in.String(), conn_in.LocalAddr().Network(), conn_in.LocalAddr().String(), conn_out.LocalAddr().Network(), conn_out.LocalAddr().String(), conn_out.RemoteAddr().Network(), conn_out.RemoteAddr().String())
							log.Println(err)
						}
						pipes_lock.Lock()
						delete(pipes, hashable_addr {
							Network: addr_in.Network(),
							String: addr_in.String(),
						})
						pipes_lock.Unlock()
						pipe_in.Close()
						if conn_out_tcp, ok := conn_out.(*net.TCPConn); ok {
							conn_out_tcp.CloseWrite()
						} else {
							conn_out.Close()
						}
					}()
					go func() {
						var err error
						var packet_len int
						buffer := make([]byte, 2048)
						if _, conn_out_is_dgram := conn_out.(net.PacketConn); conn_out_is_dgram {
							for {
								packet_len, err = conn_out.Read(buffer)
								if err != nil { break }
								_, err = conn_in.WriteTo(buffer[:packet_len], addr_in)
								if err != nil { break }
							}
						} else {
							for {
								_, err = io.ReadFull(conn_out, buffer[:2])
								if err != nil { break }
								packet_len = (int(buffer[0]) << 8) | int(buffer[1])
								if packet_len > 2046 {
									err = &tooLargePacketError {
										Size: packet_len,
									}
									break
								}
								_, err = io.ReadFull(conn_out, buffer[2:2+packet_len])
								if err != nil { break }
								_, err = conn_in.WriteTo(buffer[2:2+packet_len], addr_in)
								if err != nil { break }
							}
						}
						if err == io.EOF {
							log.Printf("%s %s <---> %s %s <=X== %s %s <---> %s %s\n", addr_in.Network(), addr_in.String(), conn_in.LocalAddr().Network(), conn_in.LocalAddr().String(), conn_out.LocalAddr().Network(), conn_out.LocalAddr().String(), conn_out.RemoteAddr().Network(), conn_out.RemoteAddr().String())
						} else {
							log.Printf("%s %s <---> %s %s <=!== %s %s <---> %s %s\n", addr_in.Network(), addr_in.String(), conn_in.LocalAddr().Network(), conn_in.LocalAddr().String(), conn_out.LocalAddr().Network(), conn_out.LocalAddr().String(), conn_out.RemoteAddr().Network(), conn_out.RemoteAddr().String())
							log.Println(err)
						}
						if conn_out_tcp, ok := conn_out.(*net.TCPConn); ok {
							conn_out_tcp.CloseRead()
						}
					}()
					pipe_out.Write(first_packet)
				}(addr_in, first_packet)
			}
			now := time.Now()
			for k, v := range pipes {
				if v.TTL.Before(now) {
					pipes_lock.Lock()
					delete(pipes, k)
					pipes_lock.Unlock()
					v.Pipe.Close()
				}
			}
		}
	}()
	return nil
}

type tooLargePacketError struct {
	Size int
}

func (e *tooLargePacketError) Error() string {
	return fmt.Sprintf("packet too large (%d > 2046)", e.Size)
}
