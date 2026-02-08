# Архитектура

## Обзор

Secret Copy Operator построен на базе [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime) (Kubebuilder). Оператор работает в management кластере и копирует секреты в удалённые workload кластеры.

```
┌─────────────────────────────────────────────────────────────┐
│                    Management Cluster                        │
│                                                              │
│  ┌──────────────┐    ┌─────────────────────────────────┐   │
│  │   Secrets    │───▶│     Secret Copy Operator        │   │
│  │ (с лейблом)  │    │                                 │   │
│  └──────────────┘    │  ┌───────────────────────────┐ │   │
│                      │  │    SecretCopyReconciler   │ │   │
│  ┌──────────────┐    │  └─────────────┬─────────────┘ │   │
│  │  Kubeconfig  │───▶│                │               │   │
│  │   Secrets    │    │  ┌─────────────▼─────────────┐ │   │
│  └──────────────┘    │  │     ClusterManager        │ │   │
│                      │  │   (кэш клиентов)          │ │   │
│                      │  └─────────────┬─────────────┘ │   │
│                      └────────────────┼───────────────┘   │
└───────────────────────────────────────┼───────────────────┘
                                        │
                    ┌───────────────────┼───────────────────┐
                    │                   │                   │
                    ▼                   ▼                   ▼
         ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐
         │ Workload        │ │ Workload        │ │ Workload        │
         │ Cluster 1       │ │ Cluster 2       │ │ Cluster N       │
         │                 │ │                 │ │                 │
         │ ┌─────────────┐ │ │ ┌─────────────┐ │ │ ┌─────────────┐ │
         │ │   Secret    │ │ │ │   Secret    │ │ │ │   Secret    │ │
         │ │  (копия)    │ │ │ │  (копия)    │ │ │ │  (копия)    │ │
         │ └─────────────┘ │ │ └─────────────┘ │ │ └─────────────┘ │
         └─────────────────┘ └─────────────────┘ └─────────────────┘
```

## Компоненты

### Структура файлов

```
internal/controller/
├── secret_controller.go    # Reconcile, copySecret
├── cluster_manager.go      # Кэш клиентов к удалённым кластерам
├── config.go               # CopyConfig, parseConfig()
├── constants.go            # Аннотации, лейблы, статусы
├── strategy.go             # Strategy тип, ParseStrategy()
└── backoff.go              # Exponential backoff логика
```

### SecretCopyReconciler

Основной контроллер, обрабатывающий события secrets.

**Обязанности:**
- Отслеживание secrets с лейблом `secret-copy.in-cloud.io=true`
- Парсинг конфигурации из аннотаций
- Проверка существования целевого namespace
- Копирование данных в целевой кластер
- Обновление статуса синхронизации

### ClusterManager

Менеджер подключений к удалённым кластерам с кэшированием:

```
internal/controller/cluster_manager.go
```

**Обязанности:**
- Создание и кэширование клиентов к удалённым кластерам
- Инвалидация кэша по TTL или при изменении kubeconfig
- Настройка rate limiting для каждого кластера

## Reconciliation Flow

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Reconcile Loop                               │
└─────────────────────────────────────────────────────────────────────┘
                                │
                                ▼
                   ┌────────────────────────┐
                   │  1. Get Secret         │
                   │     (source cluster)   │
                   └───────────┬────────────┘
                               │
                               ▼
                   ┌────────────────────────┐
                   │  2. Parse Config       │
                   │     (annotations)      │
                   └───────────┬────────────┘
                               │
                               ▼
                   ┌────────────────────────┐
                   │  3. Get Kubeconfig     │
                   │     Secret             │
                   └───────────┬────────────┘
                               │
                               ▼
                   ┌────────────────────────┐
                   │  4. Get/Create Client  │
                   │     (ClusterManager)   │
                   └───────────┬────────────┘
                               │
                               ▼
                   ┌────────────────────────┐
                   │  5. Check Namespace    │
                   │     (target cluster)   │
                   └───────────┬────────────┘
                               │
                               ▼
                   ┌────────────────────────┐
                   │  6. Copy Secret        │
                   │     (target cluster)   │
                   └───────────┬────────────┘
                               │
                               ▼
                   ┌────────────────────────┐
                   │  7. Update Status      │
                   │     (source secret)    │
                   └────────────────────────┘
```

## Event Filtering

Контроллер использует predicate для фильтрации событий:

```go
WithEventFilter(predicate.Funcs{
    CreateFunc: func(e event.CreateEvent) bool {
        return selector.Matches(labels.Set(e.Object.GetLabels()))
    },
    UpdateFunc: func(e event.UpdateEvent) bool {
        return selector.Matches(labels.Set(e.ObjectNew.GetLabels()))
    },
    DeleteFunc: func(e event.DeleteEvent) bool {
        return false  // Delete события игнорируются
    },
})
```

**Важно:** При удалении source секрета, копия в целевом кластере НЕ удаляется.

## Кэширование клиентов

ClusterManager кэширует клиенты для избежания повторного создания подключений:

```
┌─────────────────────────────────────────────────────┐
│                  ClusterManager                      │
├─────────────────────────────────────────────────────┤
│  Cache Key: namespace/name (kubeconfig secret ref)  │
│                                                     │
│  ┌─────────────────────────────────────────────┐   │
│  │  clusters/workload-1  →  client + hash + ts │   │
│  │  clusters/workload-2  →  client + hash + ts │   │
│  │  prod/staging-cluster →  client + hash + ts │   │
│  └─────────────────────────────────────────────┘   │
│                                                     │
│  Инвалидация:                                       │
│  - TTL истёк (по умолчанию 5 минут)                │
│  - Hash kubeconfig изменился                       │
└─────────────────────────────────────────────────────┘
```

## Rate Limiting

Для каждого удалённого кластера настраиваются лимиты:

```go
restConfig.QPS = float32(maxConcurrentReconciles * 5)
restConfig.Burst = maxConcurrentReconciles * 10
```

| max-concurrent-reconciles | QPS | Burst |
|---------------------------|-----|-------|
| 1 (default) | 5 | 10 |
| 5 | 25 | 50 |
| 10 | 50 | 100 |

## Security Model

### RBAC в management кластере

Оператор требует минимальные права:
- `get`, `list`, `watch`, `patch` на secrets
- `create`, `patch` на events

### RBAC в целевых кластерах

Определяется kubeconfig'ом. Рекомендуется создать ServiceAccount с правами только на secrets в нужных namespaces.

### Изоляция данных

- Kubeconfig хранится в отдельных secrets
- Каждый kubeconfig — отдельный клиент с изолированным rate limiter
- Ошибки одного кластера не влияют на другие

## Обработка ошибок

| Ошибка | Поведение |
|--------|-----------|
| Config error (неверные аннотации) | Статус Error, **без requeue** (ждём исправления пользователем) |
| Kubeconfig secret не найден | Exponential backoff: 30s → 60s → 120s → 240s → 5min max |
| Невалидный kubeconfig | Exponential backoff: 30s → 60s → 120s → 240s → 5min max |
| Целевой namespace не существует | Exponential backoff: 30s → 60s → 120s → 240s → 5min max |
| Ошибка создания/обновления | Exponential backoff: 30s → 60s → 120s → 240s → 5min max |
| Успешная синхронизация | Статус Synced, retry count сброшен |

### Exponential Backoff

Формула: `min(30s × 2^retryCount, 5min)`

Retry count хранится в аннотации `status.secret-copy.in-cloud.io/retryCount` и сохраняется при рестарте пода. При успешной синхронизации счётчик сбрасывается.
