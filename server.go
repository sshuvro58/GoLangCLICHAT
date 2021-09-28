/*
Simple chat server
Mithun Sarker(L20516233)
*/
package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

var delim = "->"

//Server interface
type Server struct {
	port     int
	userList []string
	//unread message format: delim + timestamp(nanosec) + delim + sourceUser + delim + room + textMessage
	//read message format: timestamp(nanosec) + delim + sourceUser + delim + room + textMessage
	messageDict   map[string][]string //message-user dictionary, map[user] = []message
	roomUsersDict map[string][]string //Room-users dictionary, key roomname, value users in the room
}

//Input interface for server, contains source user, destination user and the message
type Packet struct {
	SourceUser, DestUser, Message string
}

//Parser for all non command messages
func (s *Server) Parse(p Packet, _ *bool) error {
	if len(p.DestUser) > 0 { //private msg
		if contains(s.userList, p.DestUser) >= 0 {
			srcRoom := s.getRoomNameByUser(p.SourceUser)
			destRoom := s.getRoomNameByUser(p.DestUser)
			m := createNewMsgToStore(p.SourceUser, "", p.Message)
			if srcRoom != "" && srcRoom == destRoom {
				m = createNewMsgToStore(p.SourceUser, srcRoom, p.Message)
			}
			s.messageDict[p.DestUser] = append(s.messageDict[p.DestUser], m)
		} else {
			m := p.DestUser + " not found!!"
			return errors.New(m)
		}
	} else { //Broadcast
		if p.Message != "" {
			ur := s.getRoomNameByUser(p.SourceUser)
			s.broadcastInfo(p.SourceUser, ur, p.Message, true, true)
		}
	}
	return nil
}

//Display the list of current users
func (s *Server) ShowUsers(p Packet, users *[]string) error {
	r := s.getRoomNameByUser(p.SourceUser)
	if r == "" {
		for i := range s.userList {
			if p.SourceUser != s.userList[i] {
				*users = append(*users, s.userList[i])
			} else {
				*users = append(*users, "*"+s.userList[i])
			}
		}
	} else {
		*users = append(*users, "Current room: "+r)
		arr := s.roomUsersDict[r]
		for i := range arr {
			if p.SourceUser != arr[i] {
				*users = append(*users, arr[i])
			} else {
				*users = append(*users, "*"+arr[i])
			}
		}
	}
	return nil
}

//Dequeue and display messages to user
func (s *Server) ShowMessage(username string, messages *[]string) error {
	if len(username) > 0 && s.messageDict != nil {
		marr := s.messageDict[username]
		for i := range marr {
			isNew, _, u, _, m := getStateTimeUserRoomMessage(marr[i])
			if isNew {
				marr[i] = strings.Replace(marr[i], delim, "", 1)
				*messages = append(*messages, u+" -> "+m)
			}
		}
	}
	return nil
}

//Display all messages of current room the client belongs to
func (s *Server) ShowRoomMessage(username string, messages *[]string) error {
	if len(username) > 0 && s.messageDict != nil {
		r := s.getRoomNameByUser(username)
		if r == "" {
			fmt.Println(username + " did not join to any room!!")
			return errors.New(username + " did not join to any room!!")
		}
		var roomMsgArr []string
		for key, _ := range s.messageDict {
			marr := s.messageDict[key]
			for i := range marr {
				_, _, _, mr, _ := getStateTimeUserRoomMessage(marr[i])
				if mr == r {
					if strings.HasPrefix(marr[i], delim) {
						marr[i] = strings.Replace(marr[i], delim, "", 1)
					}
					roomMsgArr = append(roomMsgArr, marr[i]+delim+key)
				}
			}
		}
		sort.Strings(roomMsgArr)
		//fmt.Println(roomMsgArr)
		for i := range roomMsgArr {
			_, _, u, _, m := getStateTimeUserRoomMessage(roomMsgArr[i])
			parts := strings.Split(m, delim)
			du := parts[len(parts)-1]
			m = strings.Join(parts[:len(parts)-1], delim)
			*messages = append(*messages, u+" -> "+du+": "+m)

		}
	}
	return nil
}

//To add a client in the server
func (s *Server) AddUser(username string, _ *bool) error {
	if len(username) > 0 && username != "SERVER" {
		if -1 == contains(s.userList, username) {
			s.userList = append(s.userList, username)
			s.messageDict[username] = nil
			msg := username + " joined!!"
			fmt.Println(msg)
			s.broadcastInfo(username, "", msg, false, false)
		} else {
			fmt.Println("Duplicate user " + username + ", join request denied!!")
			return errors.New("Duplicate user!!")
		}
	} else {
		return errors.New("Invalid username!!")
	}
	return nil
}

//To remove a client from server, connection will be closed from client side
func (s *Server) RemoveUser(username string, _ *bool) error {
	if len(username) > 0 {
		idx := contains(s.userList, username)
		if idx >= 0 {
			r := s.getRoomNameByUser(username)
			if r != "" {
				var rep bool
				s.LeaveRoom(username, &rep)
			}
			s.userList[idx] = s.userList[len(s.userList)-1]
			s.userList[len(s.userList)-1] = ""
			s.userList = s.userList[:len(s.userList)-1]
			s.messageDict[username] = nil
		} else {
			fmt.Println("User: " + username + " not found!!")
			return errors.New("No user!!")
		}
	} else {
		return errors.New("Invalid username!!")
	}
	msg := username + " removed from server!!"
	fmt.Println(msg)
	s.broadcastInfo(username, "", msg, true, false)
	return errors.New(msg)
}

//To join a room
func (s *Server) JoinRoom(p Packet, reply *[]string) error {
	username := p.SourceUser
	roomname := p.DestUser
	if len(username) > 0 && len(roomname) > 0 {
		if contains(s.userList, username) >= 0 {
			r := s.getRoomNameByUser(username)
			if r != "" {
				var rep bool
				s.LeaveRoom(username, &rep)
			}
			s.roomUsersDict[roomname] = append(s.roomUsersDict[roomname], username)
			msg := username + " joined to room " + roomname
			fmt.Println(msg)
			s.broadcastInfo(username, roomname, msg, false, false)
			return errors.New(msg)
		} else {
			fmt.Println("Connect to a server first")
			return errors.New("User is not connected to a server!!")
		}
	} else {
		return errors.New("Invalid user name or room name!!")
	}
	return nil
}

//To remove an user from a room
func (s *Server) LeaveRoom(username string, reply *bool) error {
	for k, v := range s.roomUsersDict {
		i := contains(v, username)
		if i >= 0 {
			m := username + " leaved room " + k
			fmt.Println(m)
			s.broadcastInfo(username, k, m, false, false)
			v[i] = v[len(v)-1]
			v[len(v)-1] = ""
			s.roomUsersDict[k] = v[:len(v)-1]
			*reply = true
		}
	}	
	return nil
}

//To get interface IP automatically, while connected to internet
func getInterfaceIPv4() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Println(err)
		return ""
	}
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	ip := localAddr.IP.To4()
	if ip == nil {
		conn.Close()
		log.Println("Not connected to a IPv4 network!!")
		return ""
	}
	conn.Close()
	return ip.String()
}

//To determine the room name on which the user belongs to, default empty string
func (s *Server) getRoomNameByUser(username string) string {
	for k, v := range s.roomUsersDict {
		i := contains(v, username)
		if i >= 0 {
			return k
		}
	}
	return ""
}

//Broadcast user/system messages to all users
func (s *Server) broadcastInfo(username string, room string, text string, doExcludeUser bool, useUserName bool) {
	msg := createNewMsgToStore("SERVER", room, text)
	//msg := delim + getTime() + "->SERVER->->" + text
	if useUserName {
		//msg = delim + getTime() + delim + username + delim + room + delim + text
		msg = createNewMsgToStore(username, room, text)
	}
	if room == "" {
		for key, val := range s.messageDict {
			if doExcludeUser {
				if username != key {
					s.messageDict[key] = append(val, msg)
				}
			} else {
				s.messageDict[key] = append(val, msg)
			}
		}
	} else {
		uList := s.roomUsersDict[room]
		for i := range uList {
			if doExcludeUser {
				if username != uList[i] {
					s.messageDict[uList[i]] = append(s.messageDict[uList[i]], msg)
				}
			} else {
				s.messageDict[uList[i]] = append(s.messageDict[uList[i]], msg)
			}
		}
	}
}

func main() {
	s := new(Server)
	s.port = 1234
	s.messageDict = make(map[string][]string)
	s.roomUsersDict = make(map[string][]string)

	args := os.Args
	if len(args) > 2 {
		fn := strings.Split(args[0], "\\")
		fmt.Printf("Usage:\n\t%q [port]\n", fn[len(fn)-1])
		return
	}
	if len(args) == 2 {
		port, err := strconv.Atoi(args[1])
		if err == nil && port > 1000 && port < 65536 {
			s.port = port
		} else {
			fmt.Println("Invalid port")
			return
		}
	}
	startServer(s)
}

//Register and start listening to configured ip:port
func startServer(a *Server) {
	rpc.Register(a)
	rpc.HandleHTTP()
	ip := getInterfaceIPv4()
	if ip == "" {
		ip = "localhost"
		fmt.Println("Using localhost instead")
	}
	l, e := net.Listen("tcp", ip+":"+strconv.Itoa(a.port))
	if e != nil {
		log.Fatal("listen error:", e)
	}
	fmt.Printf("Server started at port %s:%d\n", ip, a.port)
	e = http.Serve(l, nil)
	if e != nil {
		log.Fatal("Error serving %q", e)
	}
}

//Utility functions below
func contains(arr []string, val string) int {
	for i, a := range arr {
		if a == val {
			return i
		}
	}
	return -1
}
func createNewMsgToStore(user string, room string, text string) string {
	if user == "SERVER" {
		room = ""
	}
	return delim + getTime() + delim + user + delim + room + delim + text
}
func getStateTimeUserRoomMessage(rawMessage string) (bool, int64, string, string, string) {
	if len(rawMessage) == 0 {
		return false, 0, "", "", ""
	}
	isNew := strings.HasPrefix(rawMessage, delim)
	if isNew {
		rawMessage = strings.Replace(rawMessage, delim, "", 1)
	}
	arr := strings.Split(rawMessage, delim)
	if len(arr) > 3 {
		t, _ := strconv.ParseInt(arr[0], 10, 64)
		return isNew, t, arr[1], arr[2], strings.Join(arr[3:], delim)
	}
	return isNew, 0, "", "", ""
}

func getTime() string {
	now := time.Now()
	return strconv.FormatInt(now.UnixNano(), 10)
}
