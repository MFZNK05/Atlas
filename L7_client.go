package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"sync"
)

var baseURL = "http://localhost:3000"

var staticPaths = []string{
	"/index.html", "/styles/main.css", "/about.html", "/js/app.js",
}

var dynamicPaths = []string{
	"/api/user", "/dynamic/data", "/api/item/123", "/dynamic/info",
}

var sessionIDs = []string{
	"session_1", "session_2", "session_3", "session_4", "session_5",
	"session_6", "session_7", "session_8", "session_9", "session_10",
}

func SendStaticRequest(wg *sync.WaitGroup) {
	defer wg.Done()
	path := staticPaths[rand.Intn(len(staticPaths))]
	url := baseURL + path
	fmt.Printf("[STATIC] Sending GET %s\n", url)
	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("[STATIC] Error: %s\n", err)
		return
	}
	defer resp.Body.Close()
	fmt.Printf("[STATIC] Status: %s\n", resp.Status)
}

func SendDynamicRequest(wg *sync.WaitGroup) {
	defer wg.Done()
	path := dynamicPaths[rand.Intn(len(dynamicPaths))]
	url := baseURL + path
	fmt.Printf("[DYNAMIC] Sending GET %s\n", url)
	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("[DYNAMIC] Error: %s\n", err)
		return
	}
	defer resp.Body.Close()
	fmt.Printf("[DYNAMIC] Status: %s\n", resp.Status)
}

func SendCookieRequest(wg *sync.WaitGroup) {
	defer wg.Done()
	allPaths := append(staticPaths, dynamicPaths...)
	path := allPaths[rand.Intn(len(allPaths))]
	session := sessionIDs[rand.Intn(len(sessionIDs))]

	url := baseURL + path
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Printf("[COOKIE] Error creating request: %s\n", err)
		return
	}
	req.AddCookie(&http.Cookie{Name: "session_id", Value: session})

	fmt.Printf("[COOKIE] Sending GET %s with session_id=%s\n", url, session)

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("[COOKIE] Error: %s\n", err)
		return
	}
	defer resp.Body.Close()
	fmt.Printf("[COOKIE] Status: %s\n", resp.Status)
}
