/*
Chat server client
*/
package main

import (
	"bufio"
	"fmt"
	"log"
	"net/rpc"
	"os"
	"strconv"
	"strings"
	"time"
)

type Packet struct {
	SourceUser, DestUser, Message string
}
type Client struct {
	user, room, serverAdds string
	clientHandle           *rpc.Client
}

//Method to pull and display messages from server
func (c *Client) getMessages() {
	var reply []string
	for {
		time.Sleep(time.Millisecond * 100)
		if c.clientHandle == nil {
			continue
		}
		call := c.clientHandle.Go("Server.ShowMessage", c.user, &reply, nil)
		r := <-call.Done
		if r == call {
			for i := range reply {
				log.Println(reply[i])
			}
		}
	}
}

//Method to take care of all the commands (rpc calls)
func handler(c *Client) {
	for {
		reader := bufio.NewReader(os.Stdin)
		line, err := reader.ReadString('\n')
		if err != nil {
			log.Printf("Error: %q\n", err)
			continue
		}
		line = strings.TrimSpace(line)
		params := strings.Fields(line)
		if len(params) > 0 {
			if params[0] != "~connect" && c.clientHandle == nil {
				log.Println("Connect to a server first!!")
				continue
			}
			if len(params[0]) > 1 && strings.HasPrefix(params[0], "@") { //Private message
				destUser := strings.TrimLeft(params[0], "@")
				msg := strings.Join(params[1:], " ")
				p := Packet{c.user, destUser, msg}
				var reply bool
				call := c.clientHandle.Go("Server.Parse", p, &reply, nil)
				replyCall := <-call.Done
				if replyCall != call {
					log.Fatal(replyCall)
				}
			} else if len(params) > 0 && strings.HasPrefix(params[0], "~") { //Commands
				if params[0] == "~list" {
					p := Packet{}
					p.SourceUser = c.user
					var reply []string
					call := c.clientHandle.Go("Server.ShowUsers", p, &reply, nil)
					replyCall := <-call.Done
					if replyCall != call {
						log.Fatal("diff reply!!")
					}
					fmt.Printf("Current users:\n%s\n", strings.Join(reply, "\n"))
				} else if params[0] == "~show" {
					var reply []string
					call := c.clientHandle.Go("Server.ShowRoomMessage", c.user, &reply, nil)
					replyCall := <-call.Done
					if replyCall != call {
						log.Fatal("diff reply!!")
					} else if len(reply) > 0 {
						fmt.Printf("All messages of current room:\n%s\n", strings.Join(reply, "\n"))
					} else {
						fmt.Println("No messgae found")
					}
				} else if params[0] == "~connect" && len(params) > 1 {
					var err error
					if c.clientHandle != nil {
						var reply bool
						call := c.clientHandle.Go("Server.RemoveUser", c.user, &reply, nil)
						replyCall := <-call.Done
						if replyCall != call {
							log.Fatal("diff reply!!")
						}
						log.Printf("%s\n", replyCall.Error)
						c.clientHandle.Close()
						c.clientHandle = nil
					}
					c.clientHandle, err = rpc.DialHTTP("tcp", params[1])
					if err != nil {
						c.clientHandle = nil
						log.Printf("Error establishing connection with host: %q", err)
						continue
					}
					c.serverAdds = params[1]
					var reply bool
					call := c.clientHandle.Go("Server.AddUser", c.user, &reply, nil)
					replyCall := <-call.Done
					if replyCall != call {
						log.Printf("Error: %q", replyCall)
						continue
					}
					log.Printf("Client Info:\n\tuser: %s\n\troom: %s\n\tserver: %s\n", c.user, c.room, c.serverAdds)
				} else if params[0] == "~quit" {
					if c.clientHandle != nil {
						var reply bool
						call := c.clientHandle.Go("Server.RemoveUser", c.user, &reply, nil)
						replyCall := <-call.Done
						log.Printf("%s\n", replyCall.Error)
						c.clientHandle.Close()
						c.clientHandle = nil
					} else {
						log.Println("Already disconnected!!")
					}
				} else if params[0] == "~join" && len(params) > 1 {
					if len(params[1]) > 0 {
						p := Packet{}
						p.SourceUser = c.user
						p.DestUser = params[1]
						var reply []string
						call := c.clientHandle.Go("Server.JoinRoom", p, &reply, nil)
						replyCall := <-call.Done
						if replyCall != call {
							log.Printf("Error: %q", replyCall)
							continue
						}
						//fmt.Printf("\n%s", replyCall.Error)
						//fmt.Printf("\n%s\n", strings.Join(reply, "\n"))
						c.room = params[1]
					} else {
						fmt.Println("Invalid room name")
					}
				} else if params[0] == "~leave" {
					var reply bool
					call := c.clientHandle.Go("Server.LeaveRoom", c.user, &reply, nil)
					replyCall := <-call.Done
					if replyCall != call {
						log.Fatal("diff reply!!")
					} else {
						if reply {
							//fmt.Printf("%s leaved existing room\n", c.user)
							c.room = ""
						} else {
							fmt.Printf("%s did not join any room\n", c.user)
						}
					}
				} else {
					str := `Available commands: starts with tilde (~)
					~connect <ServerIP:Port> - Connect to a server:port
					~quit - Close connection to a server
					~join <RoomName> - Join to a room
					~leave - Leave existing room
					~list - List up all users
					~show - Display all messages of the room
				@<username> <private message> - Send a private message to a user
				Type anything and press enter to braodcast to room(if joined) or all`
					fmt.Println(str)
				}
			} else {
				p := Packet{c.user, "", line}
				var reply bool
				parseCall := c.clientHandle.Go("Server.Parse", p, &reply, nil)
				replyCall := <-parseCall.Done
				if replyCall != parseCall {
					log.Fatal("diff reply!!")
				}
			}
		} else {
			str := `Available commands: starts with tilde (~)
		~connect <ServerIP:Port> - Connect to a server:port
		~quit - Close connection to a server
		~join <RoomName> - Join to a room
		~leave - Leave existing room
		~list - List up all users
		~show - Display all messages of the room
	@<username> <private message> - Send a private message to a user
	Type anything and press enter to braodcast to room(if joined) or all`
			fmt.Println(str)
		}
	}
}

//Method to connect to the server
func attemptToConnect(c *Client) bool {
	var err error
	if c.clientHandle != nil {
		var reply bool
		call := c.clientHandle.Go("Server.RemoveUser", c.user, &reply, nil)
		replyCall := <-call.Done
		if replyCall != call {
			log.Fatal("diff reply!!")
		}
		log.Printf("%s\n", replyCall.Error)
		c.clientHandle.Close()
		c.clientHandle = nil
	}
	c.clientHandle, err = rpc.DialHTTP("tcp", c.serverAdds)
	if err != nil {
		c.clientHandle = nil
		log.Printf("Error establishing connection with host: %q", err)
		return false
	}
	var reply bool
	call := c.clientHandle.Go("Server.AddUser", c.user, &reply, nil)
	replyCall := <-call.Done
	if replyCall != call {
		log.Printf("Error: %q", replyCall)
		return false
	}
	return true
}

func main() {
	now := time.Now()
	sec := now.Unix()
	c := &Client{"U" + strconv.Itoa(int(sec)), "", "140.158.68.2:1234", nil}
	args := os.Args
	if len(args) > 4 {
		fn := strings.Split(args[0], "\\")
		fmt.Printf("Usage:\n\t%q [username] [room] [serverIP:port]\n", fn[len(fn)-1])
		return
	}
	if len(args) == 2 {
		c.user = args[1]
	} else if len(args) == 3 {
		c.user = args[1]
		c.room = args[2]
	} else if len(args) == 4 {
		c.user = args[1]
		c.room = args[2]
		c.serverAdds = args[3]
	}
	if attemptToConnect(c) {
		fmt.Printf("Client Info:\nuser name: %s\nroom: %s\nserver: %s\n", c.user, c.room, c.serverAdds)
		if c.room != "" {
			p := Packet{}
			p.SourceUser = c.user
			p.DestUser = c.room
			var reply []string
			call := c.clientHandle.Go("Server.JoinRoom", p, &reply, nil)
			replyCall := <-call.Done
			if replyCall != call {
				log.Printf("Error: %q", replyCall)
				c.room = ""
				return
			}
			fmt.Printf("\n%s", replyCall.Error)
			fmt.Printf("\n%s\n", strings.Join(reply, "\n"))
		}
	}
	go c.getMessages()
	handler(c)
}
