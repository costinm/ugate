package cmd


import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/ollama/ollama/api"
)

type OLLama struct {
	Address string
	Prompt string
}

func RegisterOllama() {
	// ClientFromEnvironment uses OLLAMA_HOST env var.

	// My local ollama + webui in a docker
	u, _ := url.Parse("http://192.168.2.18:11434")
	client := api.NewClient(u, http.DefaultClient)

	req := &api.GenerateRequest{
		Model:  "llama3.1",
		Prompt: "how many planets are there?",

		// set streaming to false
		Stream: new(bool),
	}

	ctx := context.Background()
	respFunc := func(resp api.GenerateResponse) error {
		// Only print the response here; GenerateResponse has a number of other
		// interesting fields you want to examine.
		fmt.Println(resp.Response)

		log.Println("Eval", resp.EvalDuration, "load", resp.LoadDuration)
		return nil
	}

	t0 := time.Now()
	err := client.Generate(ctx, req, respFunc)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Gentime: ", time.Since(t0))
	respFunc2 := func(resp api.ChatResponse) error {
		// Only print the response here; GenerateResponse has a number of other
		// interesting fields you want to examine.
		fmt.Println(resp.Message.Content)

		log.Println("Eval", resp.EvalDuration, "load", resp.LoadDuration)
		return nil
	}

	client.Chat(ctx, &api.ChatRequest{}, respFunc2)

}



const (
	apiURL = "https://api.anthropic.com/v1/messages"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Request struct {
	Model     string    `json:"model"`
	Messages  []Message `json:"messages"`
	MaxTokens int       `json:"max_tokens"`
}

type Response struct {
	Content string `json:"content"`
}

// https://console.anthropic.com/dashboard
func Anhropic() {
	kb, err := os.ReadFile("~/.ssh/ANTHROPIC_API_KEY")
	apiKey := string(kb)
	if err != nil {
		fmt.Println("Please set the ANTHROPIC_API_KEY environment variable")
		return
	}

	client := &http.Client{}

	request := Request{
		Model: "claude-3-opus-20240229",
		Messages: []Message{
			{Role: "user", Content: "What is the capital of France?"},
		},
		MaxTokens: 100,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		fmt.Println("Error marshaling JSON:", err)
		return
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Println("Error creating request:", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error sending request:", err)
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response:", err)
		return
	}

	var response Response
	err = json.Unmarshal(body, &response)
	if err != nil {
		fmt.Println("Error unmarshaling JSON:", err)
		return
	}

	fmt.Println("Claude's response:", response.Content)
}
