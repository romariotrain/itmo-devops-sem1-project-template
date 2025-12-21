#!/bin/bash

set -e

echo "=== Подготовка окружения ==="

if ! command -v psql &> /dev/null; then
    echo "PostgreSQL не установлен. Устанавливаем..."
    sudo apt-get update
    sudo apt-get install -y postgresql postgresql-contrib
fi

sudo service postgresql start

echo "Создание пользователя и базы данных..."

sudo -u postgres psql <<EOF
DO \$\$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_user WHERE usename = 'validator') THEN
        CREATE USER validator WITH PASSWORD 'val1dat0r';
    END IF;
END
\$\$;
DROP DATABASE IF EXISTS "project-sem-1";

CREATE DATABASE "project-sem-1" OWNER validator;

EOF

echo "Создание таблицы prices..."

PGPASSWORD='val1dat0r' psql -h localhost -U validator -d project-sem-1 <<EOF
CREATE TABLE IF NOT EXISTS prices (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    category VARCHAR(255) NOT NULL,
    price NUMERIC(10, 2) NOT NULL,
    create_date DATE NOT NULL
);
EOF

echo "База данных успешно подготовлена!"

if command -v go &> /dev/null; then
    echo "Установка зависимостей Go..."
    go mod download
    echo "Зависимости установлены!"
else
    echo "Go не установлен. Пожалуйста, установите Go для продолжения."
fi

echo "=== Подготовка завершена ==="