# Efrem Bot

Go-приложение, которое читает личные сообщения от заданного пользователя и отправляет в чат статистику по поездкам и верификациям с указанным интервалом.

## Установка Go

### Windows
1. Скачайте установщик с официального сайта: https://go.dev/dl/
2. Запустите `.msi` и пройдите стандартный мастер установки.
3. После установки откройте `PowerShell` и проверьте версию:
   ```powershell
   go version
   ```
   Если версия не отображается, добавьте `C:\Go\bin` в переменную окружения `PATH`.

### Linux (Debian/Ubuntu)
1. Удалите старые пакеты `golang`, если они есть:
   ```bash
   sudo apt remove golang-go
   ```
2. Скачайте архив с https://go.dev/dl/ (пример для версии 1.23.1):
   ```bash
   wget https://go.dev/dl/go1.23.1.linux-amd64.tar.gz
   ```
3. Распакуйте в `/usr/local`:
   ```bash
   sudo tar -C /usr/local -xzf go1.23.1.linux-amd64.tar.gz
   ```
4. Добавьте Go в `PATH` (например, дописав в `~/.profile`):
   ```bash
   export PATH=$PATH:/usr/local/go/bin
   ```
5. Перезайдите в терминал и проверьте:
   ```bash
   go version
   ```

## Запуск

1. Скопируйте файл `.env.example` или создайте `.env` со значениями:
   - `API_ID`, `API_HASH` — ключи из [my.telegram.org](https://my.telegram.org/apps)
   - `PHONE`, `PASSWORD` — номер Telegram и пароль 2FA (если есть)
   - `SOURCE_USER` — отправитель, чьи сообщения считаем
   - `TARGET_CHAT` — чат, куда отправляем summary
   - `SUMMARY_INTERVAL` — интервал сводки (`5m`, `1h`, и т.п.)
2. Загрузите переменные и запустите:
   ```bash
   export $(grep -v '^#' .env | xargs)
   go run main.go
   ```
3. Введите код из Telegram (и пароль 2FA, если не указали в `.env`).

Через заданный интервал бот отправит сообщение вида `Поездки: N Верификации: M` и начнет новый отсчет.
