# Решение проблем

## Проверка статуса синхронизации

### Статус на source секрете

```bash
kubectl get secret my-secret -o jsonpath='{.metadata.annotations}' | jq
```

Ожидаемый вывод при успехе:
```json
{
  "status.secret-copy.in-cloud.io/lastSyncStatus": "Synced",
  "status.secret-copy.in-cloud.io/lastSyncTime": "2024-01-15T10:30:00Z"
}
```

При ошибке:
```json
{
  "status.secret-copy.in-cloud.io/lastSyncStatus": "Error: kubeconfig not found",
  "status.secret-copy.in-cloud.io/lastSyncTime": "2024-01-15T10:30:00Z"
}
```

### Проверка целевого секрета

```bash
# Используя kubeconfig целевого кластера
kubectl --kubeconfig=/path/to/target/kubeconfig get secret my-secret -n target-ns
```

## Типичные ошибки

### "kubeconfig not found"

**Причина:** Секрет с kubeconfig не найден в указанном namespace/name.

**Решение:**
1. Проверьте аннотацию `secret-copy.in-cloud.io/dstClusterKubeconfig`
2. Убедитесь что секрет существует:
   ```bash
   kubectl get secret -n clusters workload-cluster-kubeconfig
   ```
3. Проверьте права оператора на чтение секретов

### "failed to parse kubeconfig"

**Причина:** Содержимое kubeconfig невалидно.

**Решение:**
1. Проверьте что kubeconfig находится в ключе `value`:
   ```bash
   kubectl get secret workload-kubeconfig -o jsonpath='{.data.value}' | base64 -d
   ```
2. Проверьте валидность kubeconfig:
   ```bash
   kubectl get secret workload-kubeconfig -o jsonpath='{.data.value}' | base64 -d > /tmp/kubeconfig
   kubectl --kubeconfig=/tmp/kubeconfig get nodes
   ```

### "failed to create client"

**Причина:** Не удаётся подключиться к целевому кластеру.

**Решение:**
1. Проверьте доступность API сервера из management кластера
2. Проверьте что токен/сертификат не истёк
3. Проверьте network policies

### "target namespace does not exist"

**Причина:** Целевой namespace не существует в destination кластере.

**Решение:**
1. Создайте namespace в целевом кластере:
   ```bash
   kubectl --kubeconfig=/path/to/target/kubeconfig create namespace target-ns
   ```
2. Или измените аннотацию `dstNamespace` на существующий namespace:
   ```bash
   kubectl annotate secret my-secret secret-copy.in-cloud.io/dstNamespace=existing-ns --overwrite
   ```

### Секрет не копируется

**Причина:** Отсутствует лейбл или неверный формат аннотаций.

**Решение:**
1. Проверьте лейбл:
   ```bash
   kubectl get secret my-secret -o jsonpath='{.metadata.labels}'
   ```
   Должен быть `secret-copy.in-cloud.io: "true"`

2. Проверьте аннотации:
   ```bash
   kubectl get secret my-secret -o jsonpath='{.metadata.annotations}'
   ```

## Диагностика через логи

### Просмотр логов оператора

```bash
kubectl logs -n secret-copy-operator-system deployment/secret-copy-operator-controller-manager -f
```

### Фильтрация по секрету

```bash
kubectl logs -n secret-copy-operator-system deployment/secret-copy-operator-controller-manager | grep "my-secret"
```

### Уровни логирования

Для более подробных логов можно изменить настройки zap в deployment.

## Проверка RBAC

### В management кластере

```bash
kubectl auth can-i get secrets --as=system:serviceaccount:secret-copy-operator-system:secret-copy-operator-controller-manager
kubectl auth can-i patch secrets --as=system:serviceaccount:secret-copy-operator-system:secret-copy-operator-controller-manager
```

### В целевом кластере

Проверьте права ServiceAccount из kubeconfig:
```bash
kubectl --kubeconfig=/path/to/target/kubeconfig auth can-i create secrets -n target-ns
kubectl --kubeconfig=/path/to/target/kubeconfig auth can-i update secrets -n target-ns
```

## Проблемы с kubeconfig

### Формат ключа

Kubeconfig должен быть в ключе `value`, не `kubeconfig`:

```yaml
# Правильно
data:
  value: <base64-encoded-kubeconfig>

# Неправильно
data:
  kubeconfig: <base64-encoded-kubeconfig>
```

### Истёкший токен

Если используется token-based аутентификация, проверьте срок действия:

```bash
kubectl get secret workload-kubeconfig -o jsonpath='{.data.value}' | base64 -d | grep token
# Декодируйте JWT и проверьте exp claim
```

### Сертификаты

Проверьте что certificate-authority-data валиден:

```bash
kubectl get secret workload-kubeconfig -o jsonpath='{.data.value}' | base64 -d | grep certificate-authority-data | awk '{print $2}' | base64 -d | openssl x509 -text -noout
```

## Проблемы с производительностью

### Медленная синхронизация

1. Увеличьте количество воркеров:
   ```yaml
   args:
     - --max-concurrent-reconciles=10
   ```

2. Проверьте метрики rate limiting в логах

### Высокое потребление памяти

1. Проверьте количество кэшированных клиентов
2. Уменьшите TTL кэша:
   ```yaml
   args:
     - --client-cache-ttl=1m
   ```

## Отладка reconcile loop

Для понимания что происходит с конкретным секретом:

```bash
# Посмотреть события
kubectl get events --field-selector involvedObject.name=my-secret

# Посмотреть логи с timestamp
kubectl logs -n secret-copy-operator-system deployment/secret-copy-operator-controller-manager --timestamps | grep "my-secret"
```

## Exponential Backoff

При transient ошибках (kubeconfig не найден, namespace не существует, сеть недоступна) контроллер использует exponential backoff:

| Retry | Delay |
|-------|-------|
| 0 | 30s |
| 1 | 60s |
| 2 | 120s |
| 3 | 240s |
| 4+ | 5min (max) |

### Проверка текущего retry count

```bash
kubectl get secret my-secret -o jsonpath='{.metadata.annotations.status\.secret-copy\.in-cloud\.io/retryCount}'
```

### Сброс retry count

При успешной синхронизации retry count автоматически сбрасывается. Для принудительного сброса (чтобы следующий retry был через 30s):

```bash
kubectl annotate secret my-secret status.secret-copy.in-cloud.io/retryCount- --overwrite
```

### Config ошибки НЕ используют backoff

Ошибки конфигурации (неверный формат аннотаций, отсутствует обязательная аннотация) **не ретраятся автоматически**. Контроллер ждёт, пока пользователь исправит аннотации — при Update события reconcile запустится снова.
