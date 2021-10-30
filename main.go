//
//  HALLOWEEN
//  work by single token
//      - allow multiple listeners and single controller
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
var upgrader = websocket.Upgrader{
	ReadBufferSize:  10240,
	WriteBufferSize: 10240,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

//================================================================

func sendSession(w http.ResponseWriter, r *http.Request) {

	id := r.URL.Query().Get("id")
	logtag := id + ":  in: "

	mux.Lock()
	if _, present := sender_count[id]; !present {
		log.Print(logtag, "define new sender id: "+id)
		sender_count[id] = 0
	}
	sender_count[id] = sender_count[id] + 1
	var my_sender_id = sender_count[id]
	log.Printf("kk %d\n", my_sender_id)
	mux.Unlock()

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print(logtag, err)
		log.Print(logtag, "exiting")
		return
	}

	for {
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

	id := r.URL.Query().Get("id")
	logtag := id + ": out: "
	log.Print(logtag, "started")

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print(logtag, err)
		log.Print(logtag, "exiting")
		return
	}

	event_queue <- "new_getter," + id + ","
	mux.Lock()
	if _, present := getters[id]; !present {
		log.Print(logtag, "define new id: "+id)
		getters[id] = ""
	}
	mux.Unlock()

	lastv := ""
	nosend := 0
	for {
		mux.RLock()
		get_mess := getters[id]
		mux.RUnlock()
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

	listenPort := "7777"
	log.Print("Listening on Port: ", listenPort)

	go share_client() // loop managing message passing
	go monitor()      // background monitoring routine

	http.HandleFunc("/send", sendSession)                  // send session handler
	http.HandleFunc("/get", getSession)                    // get session handler
	http.Handle("/", http.FileServer(http.Dir("./html/"))) // http file server

	err2 := http.ListenAndServe(":"+listenPort, nil) // LISTEN

	// if drop out of ListenAndServe then stop
	if err2 != nil {
		panic("ListenAndServe: " + err2.Error())
	}

}
