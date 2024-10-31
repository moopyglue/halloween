//
//  HALLOWEEN
//  work by single token
//      - this is a simplifed version with out the multiple servers which allow
//        scaling to 1000's of connections and relies on a single server process
//      - there are some elemnts still visible here that alude to the original more 
//        complex, but mostly in teh choice of function name.
//      - more compelx versions had a 'central' node which shared all the running 
//        and all the active connection to them so that each listening node can 
//        maintain a connection with all the other nodes easily and know who to 
//        share controller updates with 
//

package main

import (
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var event_queue = make(chan string, 1000) // sharing client config
var mux = &sync.RWMutex{}                 // simple Read/Write mux
var getters = make(map[string]string)     // track active getter list
var sender_count = make(map[string]int)   // how many senders have connected

// upgrader used to convert https to wss connections
// this is part of how websockets work. When a https connection is upgraded 
// to a websocket it becomes an interupt driven bi-directopn communications channel

var upgrader = websocket.Upgrader{
	ReadBufferSize:  10240,
	WriteBufferSize: 10240,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

//================================================================

func sendSession(w http.ResponseWriter, r *http.Request) {

	// grab the session id and log getting started
	id := r.URL.Query().Get("id")
	logtag := id + ":  in: "

	// register a new sender
	mux.Lock()
	if _, present := sender_count[id]; !present {
		log.Print(logtag, "define new sender id: "+id)
		sender_count[id] = 0
	}
	sender_count[id] = sender_count[id] + 1
	var my_sender_id = sender_count[id]
	log.Printf("kk %d\n", my_sender_id)
	mux.Unlock()

	// upgrade the https connection to a wss (or an http to a ws) 
	// this is part of how websockets work. When a https connection is upgraded 
	// to a websocket it becomes an interupt driven bi-directopn communications channel
	// this provides very low latency client/server communication
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print(logtag, err)
		log.Print(logtag, "exiting")
		return
	}

	for {
		// Infinate loop where this routine blocks waiting on input
		// if the clonection is lost then the function returns and 
		// thread resources are freed up.

		// clients will send an updated status, even if it is the same  as before every 1000ms, or
		// every tiethe control status changes. The continious 'chat' keeps the connection healthy
		
		_, rec_mess, err := conn.ReadMessage()
		if err != nil {
			log.Print(logtag, err)
			log.Print(logtag, "exiting")
			return
		}
		event_queue <- "send," + id + "," + string(rec_mess)
		mux.RLock()
		if my_sender_id != sender_count[id] {
			mux.RUnlock()
			conn.Close()
			return
		}
		mux.RUnlock()

	}
}

//================================================================

func getSession(w http.ResponseWriter, r *http.Request) {

	// grab the session id and log getting started
	id := r.URL.Query().Get("id")
	logtag := id + ": out: "
	log.Print(logtag, "started")

	// upgrade the https connection to a wss (or an http to a ws) 
	// this is part of how websockets work. When a https connection is upgraded 
	// to a websocket it becomes an interupt driven bi-directopn communications channel
	// this provides very low latency client/server communication
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print(logtag, err)
		log.Print(logtag, "exiting")
		return
	}

	// once we have converted to a websocket 
	// we add the connection to the list of getters
	event_queue <- "new_getter," + id + ","
	mux.Lock()
	if _, present := getters[id]; !present {
		log.Print(logtag, "define new id: "+id)
		getters[id] = ""
	}
	mux.Unlock()

	// the getters always recive 'absolute positioning data' so even if they missed a few 
	// updates from the controller the latest position is always the most recent so it
	// automatically skips any flurry of fast paceed updates.
	lastv := ""
	nosend := 0
	for {
		// infinate loop to stay connected to client 
		
		mux.RLock()  // read lock as apposed to a full read/write lock
		get_mess := getters[id]
		mux.RUnlock()

		// update is sent to remote client either every time new data is recieved 
		// or every 20 'ticks' - in this case a tick is 50ms x 20 = 1000
		if get_mess != lastv || nosend > 20 {
			var err = conn.WriteMessage(1, []byte(get_mess))
			if err != nil {
				log.Print(logtag, err)
				log.Print(logtag, "exiting")
				return
			}
			if get_mess != "" || true {
				log.Print(logtag, string(get_mess))
			}
			nosend = 0
		} else {
			nosend++
		}
		time.Sleep(50 * time.Millisecond)
		lastv = get_mess
	}
}

func monitor() {

	// This is not particularly useful but during debugging it is possible to 
	// display status information every 5 seconds so the overall process can be monitored
	
	for {
		time.Sleep(5000 * time.Millisecond)
		log.Print("monitor: event_queue:", len(event_queue))
		for k1, e1 := range getters {
			log.Print("monitor: id out: " + k1 + " " + string(e1))
		}
		for k2, _ := range sender_count {
			log.Print("monitor: counts: ", k2, " ", sender_count[k2])
		}
	}
}

func share_client() {

	// this unamed inline go routine spins up an infinate loop which 
	// waits for incoming events and then copies all incoming events (control information)
	// to any currently connected listen websocket connections

	// we use 'mux' mutually exclusive locking mecahnism to avoid corruption of
	// arrays holing values that can be altered/read simultainiously by other go routines.
	
	//sec := 1000 * time.Millisecond
	stop := make(chan bool)

	go func() {
		sep := regexp.MustCompile(",")
		log.Print("share_client: starting event manager")
		for {

			s := ""
			select {
			case s = <-event_queue:
			case <-stop:
				break
			}

			log.Print("action: " + string(s))
			vals := sep.Split(string(s), -1)
			if vals[0] == "send" {
				mux.Lock()
				getters[vals[1]] = strings.Join(vals[2:], ",")
				mux.Unlock()
			}
		}
		stop <- true
	}()

	<-stop
	log.Print("share_client: stopped")

}

func main() {

	// The main route spins off a number of background go routines to 
	// handle the backgrouns threads while the main thread listens for 
	// connection and deals with them
	
	// Listen on Port 7777 by default or use LISTEN_PORT env when supplied
	listenPort := "7777"
    	listenPortEnv := os.Getenv("LISTEN_PORT")
    	if listenPortEnv != "" {
            listenPort = listenPortEnv
	}
        log.Print("Listening on Port: ", listenPort )

	// go routine : loop managing message passing from send:controller to get:displayer
	go share_client() 

	// go routine : monitors in background and logs to output
	go monitor()    

	// Set up send handler
	// when connected it maintains a go routine connection via Websockets
	// note: when we receive a send request it is the external client sending to the server not us sending to them
	http.HandleFunc("/send", sendSession)
	
	// Set up get handler
	// when connected it maintains a go routine connection via Websockets
	// note: when we receive a get request it is the external client 'getting'from the server not us getting input
	http.HandleFunc("/get", getSession)        

	// if send and get are not used then defaults to a web server which servies up the needed 
	// html etc... files which are stored in the html directory
	http.Handle("/", http.FileServer(http.Dir("./html/"))) // http file server

	// Listen on the chosen port until process is killed
	err2 := http.ListenAndServe(":"+listenPort, nil) // LISTEN
	// if drop out of ListenAndServe then stop
	if err2 != nil {
		panic("ListenAndServe: " + err2.Error())
	}

}
