package main

import (
    "bytes"
    "fmt"
    "io/ioutil"
    "encoding/json"
    "time"
    "math/rand"
)

type Message struct {
    Players int
    Round int
    Timer int
    Question string
    Answers []string
}

type QandA struct {
	Question string
	Answers []string
}

type QuestionStruct struct {
	Questions []QandA
}

var junkAs = []string{"Mexican Cheese mix", "Spotify", "Laurence Fishburne", "Joe Biden", "George W. Bush", "Adam Sandler", "Colorado Springs area",
	"The War of 1812", "Watermelon", "The Oregon Trail ","Hell’s Angels", "Harley Davidson"}

var qs QuestionStruct

// var qs = []QandA{
// 	{"Who’s da guy who directed films like To Catch a Thief, The 39 Steps, and Psycho are some of the films directed by him?",
// 	[]string{"Alfred Hitchcock", "Stanley Kubrick", "Woody Allen", "Martin Scorcese", "Tim Burton", "Wes Anderson", "Christopher Nolan", "Ridley Scott"}},
// }

func readQs(){
	content, err := ioutil.ReadFile("questions.json")
    if err!=nil{
        fmt.Print("Error:",err)
    }
    err=json.Unmarshal(content, &qs)
    if err!=nil{
        fmt.Print("Error:",err)
    }
    // fmt.Println(qs)
}

// hub maintains the set of active connections and broadcasts messages to the
// connections.
type hub struct {
	// Registered connections.
	connections map[*connection]bool

	// Inbound messages from the connections.
	broadcast chan []byte

	// Register requests from the connections.
	register chan *connection

	// Unregister requests from connections.
	unregister chan *connection
}

var h = hub{
	broadcast:   make(chan []byte),
	register:    make(chan *connection),
	unregister:  make(chan *connection),
	connections: make(map[*connection]bool),
}

var curRound = 0
var numRight = 0
var numWrong = 0
var curQuestion = "cur question"
var curAnswer = "cur answer"
var questionActive = false
var myTimer = time.NewTimer(0)
var transitionTime = 5
var questionTime = 30

func disarmTimer(){
	if(!myTimer.Stop()){
		<- myTimer.C
	}
}

func sendAll(m Message){
	for cs := range h.connections {
		select {
		case cs.send <- m:
		default:
			close(cs.send)
			delete(h.connections, cs)
		}
	}
}

func wrong(){
	questionActive = false
	numWrong++
	sendAll(Message{ len(h.connections), curRound, transitionTime, "Sorry, that isn’t right bub. But don’t fret, try this next one!", []string{curAnswer} })
	myTimer.Reset(time.Duration(transitionTime) * time.Second)
}

func nextQuestion(){
	if(numWrong == 3 || len(qs.Questions)==0){
		// game over
		numRight = 0
		numWrong = 0
		sendAll(Message{ len(h.connections), curRound, transitionTime, "Game over", []string{} })
		curRound = 0
		myTimer.Reset(time.Duration(transitionTime) * time.Second)
		return
	}
	if(numRight == 3){
		// next round
		numRight = 0
		numWrong = 0
		curRound++
		sendAll(Message{ len(h.connections), curRound, transitionTime, "Congratulations! But it’s not over yet. Prepare for the next round!", []string{} })
		myTimer.Reset(time.Duration(transitionTime) * time.Second)
		return
	}
	questionActive = true
	// pick question
	x := rand.Intn(len(qs.Questions))
	q := qs.Questions[x]
	curQuestion = q.Question
	curAnswer = q.Answers[0]
	// splitup answers and send out
	ps := len(h.connections)
	var asPer int
	switch(ps){
	case 1:
		asPer = 8
	case 2:
		fallthrough
	case 3:
		asPer = 4
	case 4:
		fallthrough
	case 5:
		fallthrough
	case 6:
		asPer = 3
	case 7:
		fallthrough
	case 8:
		asPer = 2
	default:
		asPer = 1
	}
	for len(q.Answers) < (asPer * ps){
		q.Answers = append(q.Answers, junkAs[rand.Intn(len(junkAs))])
	}
	i := rand.Intn(len(q.Answers))
	for c := range h.connections {
		subset := []string{}
		for j:= 0; j<asPer; j++ {
			subset = append(subset, q.Answers[(i+j*ps)%len(q.Answers)])
		}
		select {
		case c.send <- Message{ len(h.connections), curRound, questionTime, q.Question, subset }:
		default:
			close(c.send)
			delete(h.connections, c)
		}
		i++
	}
	myTimer.Reset(time.Duration(questionTime) * time.Second)
	qs.Questions[x], qs.Questions = qs.Questions[len(qs.Questions)-1], qs.Questions[:len(qs.Questions)-1]
}

func (h *hub) run() {
	disarmTimer()
	readQs()
	// qs.Questions = qs.Questions[:6]
	rand.Seed( time.Now().UTC().UnixNano())
	for {
		select {
		case c := <-h.register:
			h.connections[c] = true
			if(curRound > 0){
				c.send <- Message{ len(h.connections), curRound, 0, curQuestion, []string{} }
			} else {
				sendAll(Message{ len(h.connections), 0, 0, "", []string{} })
			}
		case c := <-h.unregister:
			if _, ok := h.connections[c]; ok {
				delete(h.connections, c)
				close(c.send)
			}
		case m := <-h.broadcast:
			if(curRound == 0){
				if(bytes.Equal(m, []byte("start"))){
					curRound++
					// pick question and send
					nextQuestion()
				}
			} else {
				if(questionActive){
					// check answer and logic
					disarmTimer()
					if(bytes.Equal(m, []byte(curAnswer))){
						questionActive = false
						numRight++
						sendAll(Message{ len(h.connections), curRound, transitionTime, "Congratulations! Now prepare for the next one!", []string{curAnswer} })
						myTimer.Reset(time.Duration(transitionTime) * time.Second)
					} else {
						wrong()
					}
				}
			}
		case <- myTimer.C:
			if(curRound == 0){
				sendAll(Message{ len(h.connections), 0, 0, "", []string{} })
			} else if(questionActive){
				// timed out on question
				wrong()
			} else {
				// new question
				nextQuestion()
			}
		}
	}
}
