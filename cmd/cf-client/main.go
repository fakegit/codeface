package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jingweno/codeface"
	log "github.com/sirupsen/logrus"
)

func main() {
	accessToken := os.Getenv("HEROKU_API_TOKEN")
	app := os.Getenv("HEROKU_APP")

	if accessToken == "" || app == "" {
		log.Fatalf("Provide HEROKU_API_TOKEN and HEROKU_APP")
	}

	deployer := codeface.NewDeployer(accessToken)
	url, err := deployer.Deploy(context.Background(), app, os.Stderr)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Visit %s\n", url)
}