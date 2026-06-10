package firebase

import (
	"context"
	"log"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
)

type Client struct {
	app *firebase.App
	fcm *messaging.Client
}

func NewClient(credentialsPath string) (*Client, error) {
	if credentialsPath == "" {
		log.Println("[Firebase] Warning: FIREBASE_CREDENTIALS_PATH is empty. FCM notifications will be disabled.")
		return nil, nil
	}

	ctx := context.Background()
	opt := option.WithCredentialsFile(credentialsPath)
	app, err := firebase.NewApp(ctx, nil, opt)
	if err != nil {
		return nil, err
	}

	fcmClient, err := app.Messaging(ctx)
	if err != nil {
		return nil, err
	}

	log.Println("[Firebase] FCM client initialized successfully.")
	return &Client{
		app: app,
		fcm: fcmClient,
	}, nil
}

func (c *Client) SendPttWakeup(ctx context.Context, token string, groupId string, sessionId string) error {
	if c == nil || c.fcm == nil {
		return nil
	}

	message := &messaging.Message{
		Token: token,
		Android: &messaging.AndroidConfig{
			Priority: "high",
		},
		Data: map[string]string{
			"type":      "ptt_wakeup",
			"groupId":   groupId,
			"sessionId": sessionId,
		},
	}

	_, err := c.fcm.Send(ctx, message)
	return err
}
