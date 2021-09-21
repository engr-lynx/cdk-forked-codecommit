package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"encoding/json"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/go-git/go-git/v5/plumbing/transport/http"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	secret "github.com/aws/aws-sdk-go/service/secretsmanager"
)

type Creds struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func handler(ctx context.Context) (string, error) {

	srcRepo := os.Getenv("SRC_REPO")
	destRepo := os.Getenv("DEST_REPO")
	destSecret := os.Getenv("DEST_SECRET")

	log.Printf("SrcRepo: %v DestRepo: %v DestSecret: %v", srcRepo, destRepo, destSecret)

	sess, err := session.NewSession()
	if err != nil {
		return data, fmt.Errorf("error creating session: %s", err.Error())
	}

	svc := secret.New(sess)
	credsValue, err := svc.GetSecretValue(&secret.GetSecretValueInput{
		SecretId: aws.String(destSecret),
	})
	if err != nil {
		return data, fmt.Errorf("error getting secret value: %s", err.Error())
	}

	log.Printf(*credsValue.SecretString)
	var creds Creds
	err = json.Unmarshal([]byte(*credsValue.SecretString), &creds)
	if err != nil {
		return data, fmt.Errorf("error unmarshaling secret string: %s", err.Error())
	}
	log.Printf(creds.Username)
	log.Printf(creds.Password)

	repo, err := git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
		URL:      srcRepo,
		Progress: os.Stdout,
	})
	if err != nil {
		return data, fmt.Errorf("clone of source repo failed: %s", err.Error())
	}

	remote, err := repo.CreateRemote(&config.RemoteConfig{
		Name: "aws",
		URLs: []string{destRepo},
	})
	if err != nil {
		return data, fmt.Errorf("create remote failed: %s", err.Error())
	}

	auth := &http.BasicAuth{
		Username: creds.Username,
		Password: creds.Password,
	}

	err = remote.Push(&git.PushOptions{
		RemoteName: "aws",
		Auth:       auth,
	})
	if err != nil {
		return data, fmt.Errorf("push to destination repo failed: %s", err.Error())
	}

	return data, nil
}

func main() {
	lambda.Start(handler)
}
