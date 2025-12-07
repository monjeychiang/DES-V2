package api

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"trading-core/internal/events"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func (s *Server) websocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("ws upgrade error: %v", err)
		return
	}
	defer conn.Close()

	if s.Bus == nil {
		_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"bus not ready"}`))
		return
	}

	stream, unsub := s.Bus.Subscribe(events.EventPriceTick, 100)
	defer unsub()

	for msg := range stream {
		if err := conn.WriteJSON(msg); err != nil {
			log.Printf("ws write error: %v", err)
			return
		}
	}
}
