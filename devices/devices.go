package main

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"log"
	"net/http"
	"os"
	"strings"
	"text/template"
	"time"
)

type OnlineDevice struct {
	Name string
	Devices []string
}

const (
	MessageTemplate = `📶 *Current Online Devices*
{{range .}}
👨 *{{.Name}}*{{range .Devices}}
    📱 {{.}}{{end}}
{{end}}`
	adminChatId = 0
	tgBotToken = "botId:botToken"
)

type Identity struct {
	PrimaryDevice string
	Owns []string
	Notice bool
}

var (
	logDevice *log.Logger
	logBot *log.Logger
	bot *tgbotapi.BotAPI
	PersonHasDevice = map[string]Identity{
		"Galvin": {
			// The device used to identify a person is in home or not
			PrimaryDevice: "GalvintoneXSMax",
			Owns:          []string{"GalvintoneXSMax", "Galvins-MBP", "GalvindeiPadPro", "(other devices omitted)"},
			Notice: true,
		},
	}
	DeviceAlias = map[string]string {
		"GalvintoneXSMax": "iPhone XS Max",
		"Galvins-MBP": "MacBook Pro",
		"GalvindeiPadPro": "iPad Pro",
	}
	NoticeStateMap = map[string]bool{
		"Galvin": true,
	}
	client *http.Client
)

type Device struct {
	Host string
	MAC string
	Connection string
}

func noticePersonState(person string, found bool) {
	var direction string
	if found {
		direction = "is now at"
	} else {
		direction = "has just left"
	}
	m := tgbotapi.NewMessage(adminChatId, fmt.Sprintf("*%s %s home.*", person, direction))
	m.ParseMode = tgbotapi.ModeMarkdown
	_, err := bot.Send(m)
	if err != nil {
		logBot.Printf("message send failed: %v", err)
	}
}

func updateDeviceInfo() ([]Device, error) {
	resp, err := client.Get("http://10.0.0.1")
	if err != nil {
		return nil, errors.New("http request error " + err.Error())
	}

	d, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, errors.New("goquery init error " + err.Error())
	}

	var devices []Device

	d.Find(".data tbody tr.form-row").Each(func(i int, s *goquery.Selection) {
		var elements []string
		children := s.Find("td")

		children.Each(func(i int, s *goquery.Selection) {
			t := s.Text()
			if t != "" {
				elements = append(elements, s.Text())
			}
		})

		if len(elements) != 3 {
			return
		}

		devices = append(devices, Device{
			Host:       elements[0],
			MAC:        elements[1],
			Connection: elements[2],
		})
	})

	return devices, nil
}

func detect () {
	devices, err := updateDeviceInfo()
	if err != nil {
		logDevice.Printf("cannot get device info: %v", err)
	}
	for person, identity := range PersonHasDevice {
		if identity.Notice {
			prev, ok := NoticeStateMap[person]

			found := false
			for _, device := range devices {
				if device.Host == identity.PrimaryDevice {
					found = true
				}
			}

			if ok {
				if found != prev {
					noticePersonState(person, found)
				}
			} else {
				noticePersonState(person, found)
			}
			NoticeStateMap[person] = found
		}
	}
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func main() {
	file, err := os.OpenFile("log.log", os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		panic(err)
	}
	logDevice = log.New(file, "[deviceDiscover] ", log.LstdFlags)
	logBot = log.New(file, "[tgBot] ", log.LstdFlags)

	client = &http.Client{
		Timeout: time.Second * 30,
	}

	bot, err = tgbotapi.NewBotAPI(tgBotToken)
	if err != nil {
		panic(err)
	}

	// detect every minute
	t := time.NewTicker(time.Minute)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)
	for {
		select {
		case <- t.C :
			detect()

		case update := <- updates:
			if update.Message == nil || update.Message.Chat.ID != adminChatId { // ignore any non-Message Updates and non-admin messages
				continue
			}

			if update.Message.Text == "/homedevices" {
				chatId := update.Message.Chat.ID
				msg := tgbotapi.NewMessage(chatId, "⏳ _scanning, please wait..._")
				msg.ParseMode = tgbotapi.ModeMarkdown

				sent, err := bot.Send(msg)
				if err != nil {
					logBot.Printf("message send failed: %v", err)
				}

				var devices []Device
				devices, err = updateDeviceInfo()
				if err != nil {
					edit := tgbotapi.NewEditMessageText(chatId, sent.MessageID, "😔 failed to fetch data")
					_, err = bot.Send(edit)
					if err != nil {
						logBot.Printf("message send failed: %v", err)
					}
					continue
				}

				var onlineDevices []OnlineDevice
				for person, identity := range PersonHasDevice {
					var personOnlineDevices []string
					for _, device := range devices {
						if contains(identity.Owns, device.Host) {
							name := device.Host
							alias, ok := DeviceAlias[device.Host]
							if ok {
								name = alias
							}
							personOnlineDevices = append(personOnlineDevices, name)
						}
					}

					if len(personOnlineDevices) != 0 {
						onlineDevices = append(onlineDevices, OnlineDevice{
							Name: person,
							Devices: personOnlineDevices,
						})
					}
				}

				t := template.New("homedevices")
				temp, err := t.Parse(MessageTemplate)
				if err != nil {
					panic(err)
				}
				buf := bytes.NewBufferString("")
				err = temp.Execute(buf, onlineDevices)
				if err != nil {
					panic(err)
				}

				message := buf.String()
				edit := tgbotapi.NewEditMessageText(chatId, sent.MessageID, message)
				edit.ParseMode = tgbotapi.ModeMarkdown
				_, err = bot.Send(edit)
				if err != nil {
					logBot.Printf("message send failed: %v", err)
				}
			}

			if update.Message.Text == "/showrawlist" {
				chatId := update.Message.Chat.ID
				msg := tgbotapi.NewMessage(chatId, "⏳ _scanning, please wait..._")
				msg.ParseMode = tgbotapi.ModeMarkdown

				sent, err := bot.Send(msg)
				if err != nil {
					logBot.Printf("message send failed: %v", err)
				}

				var devices []Device
				devices, err = updateDeviceInfo()
				if err != nil {
					edit := tgbotapi.NewEditMessageText(chatId, sent.MessageID, "😔 failed to fetch data")
					_, err = bot.Send(edit)
					if err != nil {
						logBot.Printf("message send failed: %v", err)
					}
					continue
				}

				var deviceNames []string

				for _, device := range devices {
					deviceNames = append(deviceNames, "- " + device.Host)
				}

				m := strings.Join(deviceNames, "\n")
				m = "`" + m + "`"

				edit := tgbotapi.NewEditMessageText(chatId, sent.MessageID, m)
				edit.ParseMode = tgbotapi.ModeMarkdown
				_, err = bot.Send(edit)
				if err != nil {
					logBot.Printf("message send failed: %v", err)
				}
			}

			log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)

		}
	}
}
