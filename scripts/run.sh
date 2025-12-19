#!/bin/bash

set -e

echo "=== Запуск приложения ==="

if ! sudo service postgresql status > /dev/null 2>&1; then
    echo "Запуск PostgreSQL..."
    sudo service postgresql start
fi

if ! PGPASSWORD='val1dat0r' psql -h localhost -U validator -d project-sem-1 -c '\q' > /dev/null 2>&1; then
    echo "Ошибка: не удается подключиться к базе данных."
    echo "Запустите сначала скрипт prepare.sh"
    exit 1
fi

echo "База данных доступна!"

echo "Сборка приложения..."
if ! go build -o server main.go; then
    echo "Application build error"
    echo "qweeqw"
    exit 1
fi

if [ ! -f "./server" ]; then
    echo "not created"
    exit 1
fi

echo "Запуск сервера на порту 8080..."
./server