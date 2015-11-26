package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/hsluo/slack-message-forward/Godeps/_workspace/src/github.com/garyburd/redigo/redis"
	"github.com/hsluo/slack-message-forward/Godeps/_workspace/src/github.com/hsluo/slack-bot"
	"github.com/hsluo/slack-message-forward/Godeps/_workspace/src/golang.org/x/net/websocket"
)

const KEY_CHANNELS = "channels"

var (
	bot slack.Bot
)

func handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		return
	}

	toChanId := r.PostFormValue("channel_id")
	toChan := r.PostFormValue("channel_name")
	text := r.PostFormValue("text")
	splits := strings.SplitN(text, " ", 2)
	fromChan, query := splits[0], splits[1]

	channels, err := bot.ChannelsList()
	if err != nil {
		fmt.Fprintln(w, err)
	} else {
		for i := range channels {
			if fromChan == channels[i].Name {
				c, err := redis.Dial("tcp", ":6379")
				if err != nil {
					log.Println(err)
					return
				}
				defer c.Close()

				c.Send("HSET", channels[i].Id, query, toChanId)
				c.Send("SADD", KEY_CHANNELS, channels[i].Id)
				c.Flush()
				_, err = c.Receive()
				if err != nil {
					log.Println(err)
				} else {
					log.Printf("registered from=%s to=%s query=%s", channels[i].Id, toChanId, query)
					fmt.Fprintf(w, "from=%s to=%s query=%s", fromChan, toChan, query)
				}
			}
		}
	}
}

func forward(incoming, outgoing chan slack.Message) {
	c, err := redis.Dial("tcp", ":6379")
	if err != nil {
		log.Println(err)
		return
	}
	defer c.Close()
	for m := range incoming {
		if m.Type != "message" {
			continue
		}
		ok, err := redis.Bool(c.Do("SISMEMBER", KEY_CHANNELS, m.Channel))
		if err != nil {
			log.Println(err)
		} else if ok {
			fwdMap, err := redis.StringMap(c.Do("HGETALL", m.Channel))
			if err != nil {
				log.Println(err)
			} else {
				for query, toChanId := range fwdMap {
					ok, err := regexp.MatchString(query, m.Text)
					if err != nil {
						log.Println(err)
					} else if ok {
						m.Channel = toChanId
						outgoing <- m
					}
				}
			}
		}
	}
}

func startRtm(token string) (chan slack.Message, chan slack.Message) {
	wsurl, err := slack.RtmStart(bot.Token)
	if err != nil {
		log.Fatal(err)
	} else {
		log.Println(wsurl)
	}

	ws, err := websocket.Dial(wsurl, "", "https://api.slack.com/")
	if err != nil {
		log.Fatal(err)
	}

	incoming := make(chan slack.Message)
	outgoing := make(chan slack.Message)

	go func() {
		for {
			m, err := slack.RtmReceive(ws)
			if err != nil {
				log.Println(err)
			} else {
				log.Printf("receive %v", m)
				incoming <- m
			}
		}
	}()

	go func() {
		for m := range outgoing {
			m.Ts = fmt.Sprintf("%f", float64(time.Now().UnixNano())/1000000000.0)
			log.Printf("send %v", m)
			if err := slack.RtmSend(ws, m); err != nil {
				log.Println(err)
			}
		}
	}()

	return incoming, outgoing
}

func startServer() {
	var (
		ip   = os.Getenv("OPENSHIFT_GO_IP")
		port = os.Getenv("OPENSHIFT_GO_PORT")
	)
	if port == "" {
		port = os.Getenv("PORT")
		if port == "" {
			port = "8080"
		}
	}
	bind := fmt.Sprintf("%s:%s", ip, port)
	log.Printf("listening on %s...", bind)
	log.Fatal(http.ListenAndServe(bind, nil))
}

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	credentials, err := slack.LoadCredentials("credentials.json")
	if err != nil {
		log.Fatal(err)
	}
	bot = credentials.Bot

	http.HandleFunc("/register", slack.ValidateCommand(
		http.HandlerFunc(handleRegister), credentials.Commands))
}

func main() {
	go forward(startRtm(bot.Token))

	startServer()
}
