package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	messagepeer "github.com/gotd/td/telegram/message/peer"
	"github.com/gotd/td/tg"
)

func main() {
	apiID, _ := strconv.Atoi(os.Getenv("API_ID"))
	apiHash := os.Getenv("API_HASH")
	sourceUser := os.Getenv("SOURCE_USER")           // username собеседника, например @danila
	targetChat := os.Getenv("TARGET_CHAT")           // username или ID чата для пересылки
	phone := os.Getenv("PHONE")                      // номер телефона аккаунта
	password := os.Getenv("PASSWORD")                // пароль 2FA, если включен
	summaryInterval := os.Getenv("SUMMARY_INTERVAL") // интервал отправки статистики
	if summaryInterval == "" {
		summaryInterval = "1h"
	}

	if apiID == 0 || apiHash == "" || sourceUser == "" || targetChat == "" || phone == "" {
		log.Fatal("Нужно задать API_ID, API_HASH, SOURCE_USER, TARGET_CHAT и PHONE")
	}

	ctx := context.Background()
	dispatcher := tg.NewUpdateDispatcher()
	client := telegram.NewClient(apiID, apiHash, telegram.Options{
		UpdateHandler: dispatcher,
	})

	err := client.Run(ctx, func(ctx context.Context) error {
		reader := bufio.NewReader(os.Stdin)
		codePrompt := func(ctx context.Context, sentCode *tg.AuthSentCode) (string, error) {
			fmt.Print("Введите код из Telegram: ")
			code, err := reader.ReadString('\n')
			if err != nil {
				return "", err
			}
			return strings.TrimSpace(code), nil
		}
		pass := password
		if pass == "" {
			fmt.Print("Введите пароль 2FA (оставьте пустым, если не требуется): ")
			rawPass, err := reader.ReadString('\n')
			if err != nil {
				return err
			}
			pass = strings.TrimSpace(rawPass)
		}
		var authenticator auth.UserAuthenticator
		if pass != "" {
			authenticator = auth.Constant(phone, pass, auth.CodeAuthenticatorFunc(codePrompt))
		} else {
			authenticator = auth.CodeOnly(phone, auth.CodeAuthenticatorFunc(codePrompt))
		}
		log.Printf("Инициируем авторизацию для %s", phone)
		if err := client.Auth().IfNecessary(ctx, auth.NewFlow(
			authenticator,
			auth.SendCodeOptions{},
		)); err != nil {
			return err
		}
		log.Println("Авторизация успешно завершена")
		api := tg.NewClient(client)
		stats := &hourlyStats{}
		interval, err := time.ParseDuration(summaryInterval)
		if err != nil {
			return fmt.Errorf("некорректное значение SUMMARY_INTERVAL: %w", err)
		}

		sourcePeer, err := resolvePeer(ctx, api, sourceUser)
		if err != nil {
			return fmt.Errorf("ошибка получения SOURCE_USER: %w", err)
		}
		targetPeer, err := resolvePeer(ctx, api, targetChat)
		if err != nil {
			return fmt.Errorf("ошибка получения TARGET_CHAT: %w", err)
		}

		log.Printf("Запускаем summary каждые %s", interval)
		ticker := time.NewTicker(interval)
		go func() {
			for {
				select {
				case <-ctx.Done():
					ticker.Stop()
					return
				case <-ticker.C:
					sendSummary(ctx, api, targetPeer, stats)
				}
			}
		}()

		dispatcher.OnNewMessage(func(ctx context.Context, _ tg.Entities, update *tg.UpdateNewMessage) error {
			message, ok := update.Message.(*tg.Message)
			if !ok || message.Out {
				return nil
			}

			if !peerMatchesInputPeer(message.PeerID, sourcePeer) || message.Message == "" {
				if message.Message != "" {
					log.Printf("Пропускаем сообщение %d от %s: источник не совпадает", message.ID, describePeer(message.PeerID))
				}
				return nil
			}

			log.Printf("Новое сообщение от собеседника: %s\n", message.Message)

			normalized := strings.ToLower(message.Message)
			if strings.Contains(normalized, "завершена поездка") {
				totalTrips := stats.AddTrip()
				log.Printf("Счетчик поездок: %d", totalTrips)
			}
			if strings.Contains(normalized, "запрос на верификацию") {
				totalVerifs := stats.AddVerification()
				log.Printf("Счетчик верификаций: %d", totalVerifs)
			}
			return nil
		})

		return telegram.RunUntilCanceled(ctx, client)
	})
	if err != nil {
		log.Fatal(err)
	}
}

type hourlyStats struct {
	mu            sync.Mutex
	trips         int
	verifications int
}

func (h *hourlyStats) AddTrip() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.trips++
	return h.trips
}

func (h *hourlyStats) AddVerification() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.verifications++
	return h.verifications
}

func (h *hourlyStats) Reset() (int, int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	trips := h.trips
	verifs := h.verifications
	h.trips = 0
	h.verifications = 0
	return trips, verifs
}

func sendSummary(ctx context.Context, api *tg.Client, target tg.InputPeerClass, stats *hourlyStats) {
	trips, verifs := stats.Reset()
	summary := fmt.Sprintf("Поездки: %d Верификации: %d", trips, verifs)
	log.Printf("Отправляем summary: %s", summary)
	if _, err := api.MessagesSendMessage(ctx, &tg.MessagesSendMessageRequest{
		Peer:     target,
		Message:  summary,
		RandomID: time.Now().UnixNano(),
	}); err != nil {
		log.Printf("Не удалось отправить summary: %v", err)
	}
}

func peerMatchesInputPeer(peer tg.PeerClass, input tg.InputPeerClass) bool {
	switch p := peer.(type) {
	case *tg.PeerUser:
		if ip, ok := input.(*tg.InputPeerUser); ok {
			return p.UserID == ip.UserID
		}
	case *tg.PeerChat:
		if ip, ok := input.(*tg.InputPeerChat); ok {
			return p.ChatID == ip.ChatID
		}
	case *tg.PeerChannel:
		if ip, ok := input.(*tg.InputPeerChannel); ok {
			return p.ChannelID == ip.ChannelID
		}
	}
	return false
}

func describePeer(p tg.PeerClass) string {
	switch peer := p.(type) {
	case *tg.PeerUser:
		return fmt.Sprintf("peerUser(%d)", peer.UserID)
	case *tg.PeerChat:
		return fmt.Sprintf("peerChat(%d)", peer.ChatID)
	case *tg.PeerChannel:
		return fmt.Sprintf("peerChannel(%d)", peer.ChannelID)
	default:
		return fmt.Sprintf("peer(%T)", p)
	}
}

func resolvePeer(ctx context.Context, api *tg.Client, username string) (tg.InputPeerClass, error) {
	username = strings.TrimPrefix(username, "@")
	res, err := api.ContactsResolveUsername(ctx, &tg.ContactsResolveUsernameRequest{
		Username: username,
	})
	if err != nil {
		return nil, err
	}
	return messagepeer.EntitiesFromResult(res).ExtractPeer(res.Peer)
}
