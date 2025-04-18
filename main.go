package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb" // Для клавиш (например, Enter)
)

// LinkResult представляет извлеченные данные о ссылке
type LinkResult struct {
	URL  string `json:"href"` // Имя поля в JS совпадает с тегом json
	Text string `json:"text"` // Имя поля в JS совпадает с тегом json
}

func main() {
	// 1. Настройка контекста и chromedp
	// Установите таймаут для всего выполнения
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel() // Гарантируем отмену контекста при выходе

	// Настройка аллокатора для запуска браузера с опциями
	// chromedp.DefaultExecAllocatorOptions[0] - это путь к исполняемому файлу Chrome по умолчанию
	// chromedp.Flag("headless", false) - Сделать браузер видимым (для отладки)
	// chromedp.Flag("headless", true) - Сделать браузер невидимым (по умолчанию)
	// Добавим user agent, чтобы выглядеть как обычный браузер
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx,
		chromedp.DefaultExecAllocatorOptions[0],
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true), // Может потребоваться в контейнерах
		chromedp.Flag("ignore-certificate-errors", true),
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/118.0.0.0 Safari/537.36"),
	)
	defer cancelAlloc()

	// Создаем новый контекст для выполнения задач chromedp
	ctx, cancel = chromedp.NewContext(allocCtx)
	defer cancel()

	// Проверка запуска браузера (опционально, но полезно)
	if err := chromedp.Run(ctx); err != nil {
		log.Fatalf("Ошибка при запуске браузера: %v", err)
	}
	log.Println("Браузер успешно запущен")

	// 2. Параметры поиска и селекторы
	searchQuery := "test" // Ваш поисковый запрос
	googleURL := "https://www.google.com/"

	// CSS-селектор для поля ввода поиска Google
	// Обратите внимание, что Google может изменить этот селектор!
	searchInputSelector := `textarea[name="q"]` // Актуально на момент написания

	// CSS-селектор для контейнера с результатами поиска
	// Это поможет понять, когда результаты загрузились
	resultsContainerSelector := `#search` // Или `#rso`

	// JavaScript код для извлечения ссылок
	// Мы ищем все <a> теги внутри контейнера результатов, которые содержат <h3> (заголовок результата).
	// Затем поднимаемся к родительскому <a> тегу (самой ссылке результата)
	// И извлекаем href и text (который берем из <h3> для точного заголовка).
	// Опять же, этот JS может нуждаться в адаптации, если Google изменит верстку.
	// Удалили "return", так как Evaluate возвращает значение последней инструкции/переменной.
	jsCode := fmt.Sprintf(`
        var links = [];
        document.querySelectorAll('%s div a h3').forEach(function(h3) {
            var link = h3.closest('a'); // Находим ближайший родительский <a>
            if (link) {
                links.push({ href: link.href, text: h3.innerText }); // Извлекаем href и текст
            }
        });
        links; // Оставляем переменную 'links', чтобы Evaluate мог получить ее значение
    `, resultsContainerSelector)

	// Переменная для хранения результатов парсинга
	var extractedLinks []LinkResult

	// 3. Выполнение задач в браузере
	log.Printf("Переход на %s", googleURL)
	err := chromedp.Run(ctx,
		// Переходим на главную страницу Google
		chromedp.Navigate(googleURL),
		// Ждем появления поля ввода поиска
		chromedp.WaitVisible(searchInputSelector, chromedp.ByQuery), // chromedp.ByQuery - для CSS-селекторов
		// Вводим поисковый запрос в поле
		chromedp.SendKeys(searchInputSelector, searchQuery, chromedp.ByQuery),
		// Нажимаем Enter для выполнения поиска
		chromedp.SendKeys(searchInputSelector, kb.Enter, chromedp.ByQuery),
		// Ждем появления контейнера с результатами поиска
		chromedp.WaitVisible(resultsContainerSelector, chromedp.ByID), // chromedp.ByID - для ID (#search)
		// (Опционально) Подождать дополнительное время, если результаты медленно загружаются
		chromedp.Sleep(5*time.Second),

		// Выполняем JavaScript для извлечения данных
		// Результат выполнения JS (массив объектов) будет декодирован в переменную extractedLinks
		chromedp.Evaluate(jsCode, &extractedLinks),
	)

	// 4. Обработка ошибок и вывод результатов
	if err != nil {
		log.Fatalf("Ошибка выполнения chromedp: %v", err)
	}

	fmt.Printf("\nНайдено %d ссылок для запроса '%s':\n", len(extractedLinks), searchQuery)
	for i, link := range extractedLinks {
		fmt.Printf("%d. Заголовок: %s\n   URL: %s\n", i+1, link.Text, link.URL)
	}
}
