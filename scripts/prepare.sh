#!/bin/bash

# Скрипт для подготовки окружения и базы данных

set -e

echo "=== Подготовка окружения ==="

# Проверка установки PostgreSQL
if ! command -v psql &> /dev/null; then
    echo "PostgreSQL не установлен. Устанавливаем..."
    sudo apt-get update
    sudo apt-get install -y postgresql postgresql-contrib
fi

# Запуск PostgreSQL, если не запущен
sudo service postgresql start

# Создание пользователя и базы данных
echo "Создание пользователя и базы данных..."

sudo -u postgres psql <<EOF
-- Создание пользователя, если не существует
DO \$\$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_user WHERE usename = 'validator') THEN
        CREATE USER validator WITH PASSWORD 'val1dat0r';
    END IF;
END
\$\$;

-- Удаление базы данных, если существует (для чистой установки)
DROP DATABASE IF EXISTS "project-sem-1";

-- Создание базы данных
CREATE DATABASE "project-sem-1" OWNER validator;

EOF

# Создание таблицы
echo "Создание таблицы prices..."

PGPASSWORD='val1dat0r' psql -h localhost -U validator -d project-sem-1 <<EOF
CREATE TABLE IF NOT EXISTS prices (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    category VARCHAR(255) NOT NULL,
    price NUMERIC(10, 2) NOT NULL,
    create_date DATE NOT NULL
);
EOF

echo "База данных успешно подготовлена!"

# Установка зависимостей Go
if command -v go &> /dev/null; then
    echo "Установка зависимостей Go..."
    go mod download
    echo "Зависимости установлены!"
else
    echo "Go не установлен. Пожалуйста, установите Go для продолжения."
fi

echo "=== Подготовка завершена ==="