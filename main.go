package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

type QuizQuestion struct {
	Question string
	Options  [4]string
	Correct  int
}

type UserState struct {
	CurrentQuestion int
	Score           int
	InQuiz          bool
}

var quizQuestions = []QuizQuestion{
	{
		Question: "Столица Франции?",
		Options:  [4]string{"Лондон", "Берлин", "Париж", "Мадрид"},
		Correct:  2,
	},
	{
		Question: "Сколько планет в Солнечной системе?",
		Options:  [4]string{"7", "8", "9", "10"},
		Correct:  1,
	},
	{
		Question: "Какой язык программирования самый лучший?",
		Options:  [4]string{"Python", "Java", "Go", "C#"},
		Correct:  3,
	},
	{
		Question: "Какое животное является символом России?",
		Options:  [4]string{"Медведь", "Орел", "Волк", "Тигр"},
		Correct:  0,
	},
	{
		Question: "В каком году человек полетел в космос?",
		Options:  [4]string{"1957", "1961", "1969", "1975"},
		Correct:  1,
	},
}

var userStates = make(map[int64]*UserState)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Ошибка загрузки .env файла")
	}

	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		log.Fatal("Токен бота не найден! Установите переменную TELEGRAM_BOT_TOKEN")
	}

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Panic("Ошибка создания бота:", err)
	}

	bot.Debug = true
	log.Printf("Бот %s запущен!", bot.Self.UserName)

	_, err = bot.Request(tgbotapi.DeleteWebhookConfig{})
	if err != nil {
		log.Printf("Не удалось удалить webhook: %v", err)
	} else {
		log.Printf("Webhook очищен")
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	u.Offset = 0
	u.Limit = 100

	updates := bot.GetUpdatesChan(u)
	log.Printf("Ожидаю сообщения... (напишите боту /start)")

	for update := range updates {
		if update.Message != nil {
			log.Printf("ПОЛУЧЕНО: [%s] %s",
				update.Message.From.UserName,
				update.Message.Text)
			handleMessage(bot, update.Message)
		}

		if update.CallbackQuery != nil {
			log.Printf("CALLBACK: %s", update.CallbackQuery.Data)
			handleCallback(bot, update.CallbackQuery)
		}
	}
}

func handleMessage(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	log.Printf("[%s] %s", message.From.UserName, message.Text)

	switch {
	case message.IsCommand() && message.Command() == "start":
		sendWelcomeMessage(bot, message.Chat.ID)

	case message.IsCommand() && message.Command() == "quiz":
		startQuiz(bot, message.Chat.ID)

	case message.IsCommand() && message.Command() == "score":
		showScore(bot, message.Chat.ID)

	case message.Text == "Начать викторину":
		startQuiz(bot, message.Chat.ID)

	case message.Text == "Мой счет":
		showScore(bot, message.Chat.ID)

	case message.Text == "Завершить викторину":
		endQuiz(bot, message.Chat.ID)
	}
}

func sendWelcomeMessage(bot *tgbotapi.BotAPI, chatID int64) {
	text := "Добро пожаловать в викторину!\n" +
		"Проверьте свои знания в разных областях.\n" +
		"Каждый вопрос имеет 4 варианта ответа.\n" +
		"Команды:\n" +
		"/quiz - начать викторину.\n" +
		"/score - посмотреть свой счет. \n" +
		"Или используйте кнопки ниже:\n"

	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Начать викторину"),
			tgbotapi.NewKeyboardButton("Мой счет"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}

func startQuiz(bot *tgbotapi.BotAPI, chatID int64) {
	userStates[chatID] = &UserState{
		CurrentQuestion: 0,
		Score:           0,
		InQuiz:          true,
	}

	sendQuestion(bot, chatID, 0)
}

func sendQuestion(bot *tgbotapi.BotAPI, chatID int64, questionIndex int) {
	log.Printf("Отправка вопроса %d для пользователя %d", questionIndex, chatID)

	if questionIndex >= len(quizQuestions) {
		log.Printf("Все вопросы завершены")
		endQuiz(bot, chatID)
		return
	}

	question := quizQuestions[questionIndex]

	var rows [][]tgbotapi.InlineKeyboardButton
	for i, option := range question.Options {
		callbackData := fmt.Sprintf("answer_%d_%d", questionIndex, i)
		button := tgbotapi.NewInlineKeyboardButtonData(option, callbackData)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(button))
	}

	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("Завершить викторину", "end_quiz"),
	))

	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)

	text := fmt.Sprintf("Вопрос %d/%d:\n%s", questionIndex+1, len(quizQuestions), question.Question)
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = keyboard

	_, error := bot.Send(msg)
	if error != nil {
		log.Printf("Ошибка отправки вопроса: %v", error)
	} else {
		log.Printf("Вопрос %d отправлен успешно", questionIndex)
	}
}

func handleCallback(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery) {
	chatID := callback.Message.Chat.ID
	messageID := callback.Message.MessageID

	switch {
	case strings.HasPrefix(callback.Data, "answer_"):
		handleAnswer(bot, chatID, messageID, callback)
	case callback.Data == "end_quiz":
		endQuiz(bot, chatID)
		response := tgbotapi.NewCallback(callback.ID, "Викторина завершена")
		bot.Send(response)
	case callback.Data == "restart_quiz":
		startQuiz(bot, chatID)
		response := tgbotapi.NewCallback(callback.ID, "Викторина начата заново!")
		bot.Send(response)

	}
}

func handleAnswer(bot *tgbotapi.BotAPI, chatID int64, messageID int, callback *tgbotapi.CallbackQuery) {
	parts := strings.Split(callback.Data, "_")
	if len(parts) < 3 {
		return
	}

	//номер вопроса
	questionIndex, _ := strconv.Atoi(parts[1])

	//номер ответа
	answerIndex, _ := strconv.Atoi(parts[2])

	state, exists := userStates[chatID]
	if !exists || !state.InQuiz {
		return
	}

	question := quizQuestions[questionIndex]
	isCorrect := answerIndex == question.Correct

	if isCorrect {
		state.Score++
	}

	var resultText string

	if isCorrect {
		resultText = "Правильно! " + getEncouragement()
	} else {
		correctAnswer := question.Options[question.Correct]
		resultText = fmt.Sprintf("Неправильно. Правильный ответ: %s", correctAnswer)
	}

	response := tgbotapi.NewCallback(callback.ID, resultText)
	bot.Send(response)

	resultText = fmt.Sprintf("Вопрос %d/%d: \n%s\n\n%s",
		questionIndex+1, len(quizQuestions),
		question.Question, resultText)

	editMsg := tgbotapi.NewEditMessageText(chatID, messageID, resultText)
	bot.Send(editMsg)

	state.CurrentQuestion++

	if state.CurrentQuestion < len(quizQuestions) {
		go func() {
			time.Sleep(2 * time.Second)
			sendQuestion(bot, chatID, state.CurrentQuestion)
		}()
	} else {
		endQuiz(bot, chatID)
	}
}

func endQuiz(bot *tgbotapi.BotAPI, chatID int64) {
	state, exists := userStates[chatID]
	if !exists {
		return
	}

	state.InQuiz = false

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Пройти еще раз", "restart_quiz"),
		),
	)

	itog := (state.Score * 100) / len(quizQuestions)
	text := fmt.Sprintf("Викторина завершена! \n\n"+
		"Ваш результат: %d/%d правильных ответов\n"+
		"Процент правильных ответов: %d%%\n\n"+
		"%s",
		state.Score, len(quizQuestions), itog, getFinalMessage(itog))

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}

func showScore(bot *tgbotapi.BotAPI, chatID int64) {
	state, exists := userStates[chatID]

	if !exists || state.Score == 0 {
		msg := tgbotapi.NewMessage(chatID, "Вы еще не проходили викторину! Используйте /quiz чтобы начать.")
		bot.Send(msg)
		return
	}

	percentage := (state.Score * 100) / len(quizQuestions)
	text := fmt.Sprintf("Ваш лучший результат: %d/%d\n. Процент правильных: %d%%\n%s",
		state.Score, len(quizQuestions), percentage, getFinalMessage(percentage))
	msg := tgbotapi.NewMessage(chatID, text)
	bot.Send(msg)
}

func getEncouragement() string {
	encouragements := []string{
		"Отлично!",
		"Превосходно!",
		"Ай да ты!",
		"Нормалек!",
		"Великолепно!",
	}

	return encouragements[rand.Intn(len(encouragements))]
}

func getFinalMessage(percentage int) string {
	switch {
	case percentage >= 90:
		return "Вы гений! Идеально!"
	case percentage >= 70:
		return "Отличный результат! Вы хорошо разбираетесь в теме!"
	case percentage >= 50:
		return "Хороший результат! Есть куда стремиться!"
	default:
		return "Не сдавайтесь! Попробуйте еще раз!"
	}
}
