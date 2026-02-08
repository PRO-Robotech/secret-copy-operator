# Secret Copy Operator

Kubernetes контроллер для копирования секретов между кластерами. Отслеживает секреты с определённым лейблом в management-кластере и копирует их в удалённые workload-кластеры используя kubeconfig.

## Сценарии использования

- Распространение TLS сертификатов от cert-manager в workload кластеры
- Шаринг учётных данных БД между кластерами
- Репликация конфигурационных секретов в разные окружения

## Быстрый старт

### 1. Разверните оператор

```bash
kubectl apply -k config/default
```

### 2. Создайте секрет с kubeconfig целевого кластера

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: workload-cluster-kubeconfig
  namespace: clusters
type: Opaque
stringData:
  value: |
    apiVersion: v1
    kind: Config
    clusters:
    - cluster:
        server: https://workload.example.com:6443
        certificate-authority-data: <base64-ca>
      name: workload
    # ... остальной kubeconfig
```

### 3. Добавьте лейблы и аннотации к секрету

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: my-secret
  namespace: default
  labels:
    secret-copy.in-cloud.io: "true"
  annotations:
    secret-copy.in-cloud.io/dstClusterKubeconfig: "clusters/workload-cluster-kubeconfig"
    secret-copy.in-cloud.io/dstNamespace: "target-namespace"
type: Opaque
data:
  key: dmFsdWU=
```

Оператор автоматически скопирует секрет в целевой кластер.

## Конфигурация

### Лейблы

| Лейбл | Значение | Описание |
|-------|----------|----------|
| `secret-copy.in-cloud.io` | `true` | Включить копирование для этого секрета |

### Аннотации

| Аннотация | Обязательная | Описание |
|-----------|--------------|----------|
| `secret-copy.in-cloud.io/dstClusterKubeconfig` | Да | Ссылка на секрет с kubeconfig (`namespace/name`) |
| `secret-copy.in-cloud.io/dstNamespace` | Нет | Целевой namespace (по умолчанию — исходный) |
| `secret-copy.in-cloud.io/dstType` | Нет | Тип секрета в целевом кластере (по умолчанию — тип исходного) |
| `strategy.secret-copy.in-cloud.io/ifExist` | Нет | `overwrite` (по умолчанию) или `ignore` |
| `fields.secret-copy.in-cloud.io/<srcKey>` | Нет | Маппинг исходного ключа на целевой |

### Маппинг полей

Копирование только определённых полей с опциональным переименованием:

```yaml
annotations:
  # Копировать tls.crt как есть
  fields.secret-copy.in-cloud.io/tls.crt: "tls.crt"
  # Переименовать pg-user в username
  fields.secret-copy.in-cloud.io/pg-user: "username"
```

## Статус синхронизации

Оператор записывает статус в аннотации исходного секрета:

```yaml
annotations:
  status.secret-copy.in-cloud.io/lastSyncTime: "2024-01-15T10:30:00Z"
  status.secret-copy.in-cloud.io/lastSyncStatus: "Synced"  # или "Error: <сообщение>"
```

## Документация

- [Справочник по конфигурации](docs/configuration.md)
- [Руководство по деплою](docs/deployment.md)
- [Архитектура](docs/architecture.md)
- [Решение проблем](docs/troubleshooting.md)
- [FAQ](docs/faq.md)

## Разработка

См. [CONTRIBUTING.md](CONTRIBUTING.md) для настройки окружения разработки.

```bash
# Запуск локально
make run

# Запуск тестов
make test

# Сборка образа
make docker-build IMG=your-registry/secret-copy-operator:tag
```

## Лицензия

Apache License 2.0
