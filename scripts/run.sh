#!/bin/bash

# Скрипт для запуска приложения

set -e

echo "=== Запуск приложения ==="

# Проверка, что PostgreSQL запущен
if ! sudo service postgresql status > /dev/null 2>&1; then
    echo "Запуск PostgreSQL..."
    sudo service postgresql start
fi

# Проверка подключения к базе данных
if ! PGPASSWORD='val1dat0r' psql -h localhost -U validator -d project-sem-1 -c '\q' > /dev/null 2>&1; then
    echo "Ошибка: не удается подключиться к базе данных."
    echo "Запустите сначала скрипт prepare.sh"
    exit 1
fi

echo "База данных доступна!"

# Сборка и запуск приложения
echo "Сборка приложения..."
if ! go build -o server main.go; then
    echo "Ошибка в сборке приложения"
    exit 1
fi

if [ ! -f "./server" ]; then
    echo "Файл не создан"
    exit 1
fi

echo "Запуск сервера на порту 8080..."
./server
