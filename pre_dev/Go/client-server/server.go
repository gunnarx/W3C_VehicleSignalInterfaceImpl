package main

import (
	"github.com/gorilla/websocket"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
)

// #include <stdlib.h>
// #include <stdio.h>
// #include <stdbool.h>
// #include "vssparserutilities.h"
import "C"

import "unsafe"

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

var nodeHandle C.long

func printInComingSession(r *http.Request) {

	Info.Printf("Method : %s \n", r.Method)
	Info.Printf("Host : %s \n", r.Host)
	Info.Printf("Proto : %s \n", r.Proto)
	Info.Printf("Uri : %s \n", r.RequestURI)

	Trace.Printf("upgrade to : %s \n", r.Header.Get("Upgrade"))

}

func getSpeed() int {
	return rand.Intn(250)
}

func getMatches(path string) int {
	// call int VSSSimpleSearch(char* searchPath, long rootNode, bool wildcardAllDepths);
	cpath := C.CString(path)
	var matches C.int = C.VSSSimpleSearch(cpath, nodeHandle, false)
	C.free(unsafe.Pointer(cpath))
	return int(matches)
}

func wsClientSession(conn *websocket.Conn) {
	defer conn.Close() // ???
	for {
		// Read message from browser
		msgType, msg, err := conn.ReadMessage()
		if err != nil {
			Error.Println("read:", err)
			break
		}

		var matches int = getMatches(string(msg))
		// Print the message to the console
		Trace.Printf("%s request: %s \n", conn.RemoteAddr(), string(msg))

		// Write message back to browser
		message := "Response:Nr of matches= " + strconv.Itoa(matches)
		response := []byte(message)

		err = conn.WriteMessage(msgType, response);
		if err != nil {
			Error.Println("write:", err)
			break
		}
	}
}

// removes initial slash, replaces following slashes with dot
func urlToPath(url string) string {
	var path string = strings.TrimPrefix(strings.Replace(url, "/", ".", -1), ".")
	return path
}

func rootServer(w http.ResponseWriter, r *http.Request) {

	printInComingSession(r)
	//check here if we should upgrade our connection to a websocket...
	if r.Header.Get("Upgrade") == "websocket" {
		Trace.Println("we are upgrading to a websocket connection")
		upgrader.CheckOrigin = func(r *http.Request) bool { return true }
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			Error.Println("upgrade:", err)
			return
		}
		go wsClientSession(conn)
		Trace.Println("WS client session spawned.")
	} else {

		var path string = urlToPath(r.RequestURI)
		var matches int = getMatches(path)

		// build a JSON string of the response, makes it easier to test against.
		message := `{"response":` + strconv.Itoa(matches) + "}"
		response := []byte(message)

		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Write(response)
		Trace.Printf("HTTP client request served for path=%s\n", path)
	}
}

func initVssFile() bool{
	filePath := "vss_rel_1.0.cnative"
	cfilePath := C.CString(filePath)
	nodeHandle = C.VSSReadTree(cfilePath)
	C.free(unsafe.Pointer(cfilePath))

	if (nodeHandle == 0) {
		Error.Println("Tree file not found.")
		return false
	}

	nodeName := C.GoString(C.getName(nodeHandle))
	Trace.Printf("Root node name=%s\n", nodeName)

	return true
}

func main() {
	setDefaultTraceLogger() // use only Trace
	http.HandleFunc("/", rootServer) // register handler

	if !initVssFile(){
		Error.Println(" Tree file not found")
		return
	}

	Error.Println(http.ListenAndServe("localhost:8080", nil)) // start server
}
