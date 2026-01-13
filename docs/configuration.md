# Справочник по конфигурации

## Аннотации секрета

### Обязательные

| Аннотация | Описание | Пример |
|-----------|----------|--------|
| `secret-copy.in-cloud.io/dstClusterKubeconfig` | Ссылка на секрет с kubeconfig целевого кластера в формате `namespace/name` | `clusters/workload-kubeconfig` |

### Опциональные

| Аннотация | По умолчанию | Описание |
|-----------|--------------|----------|
| `secret-copy.in-cloud.io/dstNamespace` | Namespace исходного секрета | Целевой namespace в удалённом кластере |
| `strategy.secret-copy.in-cloud.io/ifExist` | `overwrite` | Стратегия при существовании секрета: `overwrite` или `ignore` |

### Маппинг полей

Аннотации вида `fields.secret-copy.in-cloud.io/<srcKey>: <dstKey>` позволяют:
- Копировать только определённые поля
- Переименовывать поля при копировании

```yaml
annotations:
  # Копировать только эти поля
  fields.secret-copy.in-cloud.io/tls.crt: "tls.crt"
  fields.secret-copy.in-cloud.io/tls.key: "tls.key"
  # ca.crt НЕ будет скопирован
```

```yaml
annotations:
  # Переименование при копировании
  fields.secret-copy.in-cloud.io/pg-user: "username"       # pg-user → username
  fields.secret-copy.in-cloud.io/pg-password: "password"   # pg-password → password
```

**Важно:** Если указан хотя бы один маппинг полей, копируются ТОЛЬКО указанные поля.

## Лейблы

| Лейбл | Значение | Описание |
|-------|----------|----------|
| `secret-copy.in-cloud.io` | `true` | Обязательный лейбл для включения копирования |

## Стратегии синхронизации

### `overwrite` (по умолчанию)

Если секрет существует в целевом кластере — он будет перезаписан.

```yaml
annotations:
  strategy.secret-copy.in-cloud.io/ifExist: "overwrite"
```

### `ignore`

Если секрет уже существует — пропустить копирование. Полезно для начальной инициализации.

```yaml
annotations:
  strategy.secret-copy.in-cloud.io/ifExist: "ignore"
```

## Формат kubeconfig секрета

Секрет с kubeconfig должен содержать ключ `value` с полным содержимым kubeconfig:

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
        server: https://api.workload.example.com:6443
        certificate-authority-data: LS0tLS1CRUdJTi...
      name: workload
    contexts:
    - context:
        cluster: workload
        user: admin
      name: workload
    current-context: workload
    users:
    - name: admin
      user:
        token: eyJhbGciOiJSUzI1NiIs...
```

## CLI флаги оператора

| Флаг | По умолчанию | Описание |
|------|--------------|----------|
| `--metrics-bind-address` | `0` (выключено) | Адрес для метрик |
| `--health-probe-bind-address` | `:8081` | Адрес для health/ready проб |
| `--leader-elect` | `false` | Включить leader election для HA |
| `--client-cache-ttl` | `5m` | TTL кэша клиентов к удалённым кластерам |
| `--max-concurrent-reconciles` | `1` | Количество параллельных воркеров |
| `--cluster-name` | `system` | Имя source кластера (записывается в аннотации) |
| `--metrics-secure` | `true` | Использовать HTTPS для метрик |

## Статус-аннотации

Оператор записывает статус синхронизации в исходный секрет:

| Аннотация | Описание |
|-----------|----------|
| `status.secret-copy.in-cloud.io/lastSyncTime` | Время последней синхронизации (RFC3339) |
| `status.secret-copy.in-cloud.io/lastSyncStatus` | `Synced` или `Error: <сообщение>` |
| `status.secret-copy.in-cloud.io/retryCount` | Счётчик retry для exponential backoff (удаляется при успехе) |

## Аннотации на целевом секрете

При копировании оператор добавляет аннотации к целевому секрету:

| Аннотация | Описание |
|-----------|----------|
| `secret-copy.in-cloud.io/sourceCluster` | Имя source кластера (из флага `--cluster-name`) |
| `secret-copy.in-cloud.io/sourceSecret` | `namespace/name` исходного секрета |
| `secret-copy.in-cloud.io/copiedAt` | Время копирования (RFC3339) |
