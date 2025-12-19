# Финальный проект 1 семестра

REST API сервис для загрузки и выгрузки данных о ценах.

## Установка и запуск

### 1. Подготовка окружения

```bash
chmod +x scripts/*.sh
./scripts/prepare.sh
```

Скрипт автоматически:
- Установит PostgreSQL
- Создаст пользователя, базу данных и таблицу
- Установит зависимости

### 2. Запуск приложения

```bash
./scripts/run.sh
```

Сервер запустится на `http://localhost:8080`

### 3. Тестирование

В отдельном терминале:

```bash
./scripts/tests.sh 1
```

## API

### POST /api/v0/prices

Загрузка данных из zip-архива.

**Запрос:**
```bash
curl -X POST http://localhost:8080/api/v0/prices -F "file=@sample_data.zip"
```

**Ответ:**
```json
{
  "total_items": 10,
  "total_categories": 5,
  "total_price": 3500
}
```

### GET /api/v0/prices

Выгрузка всех данных в zip-архиве.

**Запрос:**
```bash
curl -X GET http://localhost:8080/api/v0/prices -o data.zip
```

**Ответ:** zip-архив с файлом `data.csv`

## Структура данных

**Формат CSV:**
```csv
id,name,category,price,create_date
1,item1,cat1,100,2024-01-01
```

**Таблица БД `prices`:**
- `id` (VARCHAR) - ID продукта
- `name` (VARCHAR) - название
- `category` (VARCHAR) - категория
- `price` (NUMERIC) - цена
- `create_date` (DATE) - дата создания

## Контакт

По вопросам обращайтесь к th @romix_m
