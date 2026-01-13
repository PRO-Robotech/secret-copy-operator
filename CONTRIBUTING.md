# Руководство для разработчиков

## Требования

- Go 1.23+
- Docker
- kubectl
- Kind (для E2E тестов)

## Настройка окружения

```bash
# Клонирование репозитория
git clone https://github.com/your-org/secret-copy-operator.git
cd secret-copy-operator

# Установка зависимостей
go mod download

# Установка инструментов
make controller-gen
make envtest
make golangci-lint
make mockgen
```

## Локальная разработка

### Запуск оператора локально

```bash
# Оператор подключится к текущему kubeconfig контексту
make run
```

### Запуск с кастомными флагами

```bash
go run ./cmd/main.go \
  --max-concurrent-reconciles=5 \
  --client-cache-ttl=1m \
  --cluster-name=dev
```

## Тестирование

### Unit тесты

```bash
# Запуск всех unit тестов
make test

# Запуск тестов конкретного пакета
go test ./internal/controller/... -v

# Запуск конкретного теста
go test -run TestParseConfig ./internal/controller/...

# С покрытием
go test ./internal/controller/... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### E2E тесты

```bash
# Создаёт Kind кластер и запускает E2E тесты
make test-e2e

# Только создать кластер
make setup-test-e2e

# Очистить кластер
make cleanup-test-e2e
```

## Линтинг

```bash
# Проверка
make lint

# Автоисправление
make lint-fix

# Проверка конфигурации линтера
make lint-config
```

## Генерация кода

### RBAC и манифесты

```bash
make manifests
```

### DeepCopy методы

```bash
make generate
```

### Моки для тестов

```bash
make generate-mocks
```

После изменения интерфейсов в `internal/controller/interfaces.go` необходимо перегенерировать моки.

## Структура проекта

```
.
├── cmd/main.go                 # Точка входа
├── internal/controller/
│   ├── interfaces.go           # Интерфейсы для мокирования
│   ├── secret_controller.go    # Основная логика reconcile
│   ├── cluster_manager.go      # Кэширование клиентов
│   └── *_test.go              # Unit тесты
├── config/
│   ├── manager/               # Deployment манифесты
│   ├── rbac/                  # RBAC манифесты
│   └── samples/               # Примеры использования
├── test/
│   ├── e2e/                   # E2E тесты
│   ├── mocks/                 # Сгенерированные моки
│   └── utils/                 # Утилиты для тестов
└── docs/                      # Документация
```

## Стиль кода

- Форматирование: `go fmt`
- Линтер: golangci-lint (конфигурация в `.golangci.yml`)
- Тесты: Ginkgo/Gomega

## Коммиты

Используем conventional commits:

```
feat: добавить поддержку нескольких целевых кластеров
fix: исправить утечку памяти в ClusterManager
docs: обновить README
test: добавить тесты для parseConfig
refactor: упростить логику copySecret
```

## Pull Request процесс

1. Создайте feature branch от `main`
2. Внесите изменения
3. Убедитесь что тесты проходят: `make test`
4. Убедитесь что линтер не ругается: `make lint`
5. Создайте PR с описанием изменений

### Чеклист для PR

- [ ] Тесты добавлены/обновлены
- [ ] Документация обновлена (если нужно)
- [ ] `make test` проходит
- [ ] `make lint` проходит
- [ ] Commit messages соответствуют conventional commits

## Сборка образа

```bash
# Локальная сборка
make docker-build IMG=secret-copy-operator:dev

# Multi-platform сборка
make docker-buildx IMG=your-registry/secret-copy-operator:tag
```

## Релиз

1. Обновите версию в манифестах
2. Создайте тег: `git tag v0.1.0`
3. Запушьте тег: `git push origin v0.1.0`
4. CI соберёт и опубликует образ

## Полезные команды

```bash
# Форматирование
go fmt ./...

# Проверка vet
go vet ./...

# Обновление зависимостей
go mod tidy

# Генерация всего
make generate manifests generate-mocks

# Полная проверка перед коммитом
make test lint
```
