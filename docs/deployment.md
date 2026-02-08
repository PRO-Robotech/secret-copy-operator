# Руководство по деплою

## Требования

- Kubernetes кластер 1.26+
- kubectl с доступом к management кластеру
- RBAC права на создание ClusterRole, ServiceAccount, Deployment

## Установка

### Через Kustomize

```bash
# Установка с дефолтными настройками
kubectl apply -k config/default

# Проверка статуса
kubectl get pods -n secret-copy-operator-system
```

### Через манифесты

```bash
# Сгенерировать единый манифест
make build-installer IMG=your-registry/secret-copy-operator:tag

# Применить
kubectl apply -f dist/install.yaml
```

## Настройка RBAC

Оператор требует следующие права:

```yaml
# На secrets в management кластере
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["get", "list", "watch", "patch"]

# На events для записи событий
- apiGroups: [""]
  resources: ["events"]
  verbs: ["create", "patch"]
```

Для целевых кластеров права определяются kubeconfig'ом. Рекомендуемые права:

```yaml
# В целевом кластере
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["get", "create", "update"]
```

## Настройка ресурсов

По умолчанию установлены консервативные лимиты:

```yaml
resources:
  limits:
    cpu: 500m
    memory: 128Mi
  requests:
    cpu: 10m
    memory: 64Mi
```

Для большого количества секретов увеличьте лимиты:

```yaml
# config/manager/manager.yaml
resources:
  limits:
    cpu: "1"
    memory: 256Mi
  requests:
    cpu: 100m
    memory: 128Mi
```

## High Availability

Для HA включите leader election:

```yaml
# config/manager/manager.yaml
args:
  - --leader-elect=true
```

Увеличьте количество реплик:

```yaml
spec:
  replicas: 2
```

## Настройка метрик

### Включение метрик

```yaml
args:
  - --metrics-bind-address=:8443
  - --metrics-secure=true
```

### Интеграция с Prometheus

ServiceMonitor для Prometheus Operator:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: secret-copy-operator
  namespace: secret-copy-operator-system
spec:
  endpoints:
  - port: https
    scheme: https
    tlsConfig:
      insecureSkipVerify: true
    bearerTokenFile: /var/run/secrets/kubernetes.io/serviceaccount/token
  selector:
    matchLabels:
      control-plane: controller-manager
```

## Настройка параллелизма

Для обработки большого количества секретов:

```yaml
args:
  - --max-concurrent-reconciles=10
```

При увеличении воркеров автоматически увеличиваются QPS/Burst для API клиентов:
- QPS = max-concurrent-reconciles × 5
- Burst = max-concurrent-reconciles × 10

## Настройка кэширования

TTL кэша клиентов к удалённым кластерам:

```yaml
args:
  - --client-cache-ttl=10m
```

Увеличьте для стабильных окружений, уменьшите если kubeconfig часто меняется.

## Настройка имени кластера

Для идентификации source кластера в аннотациях:

```yaml
args:
  - --cluster-name=prod-management
```

## Проверка работоспособности

```bash
# Статус пода
kubectl get pods -n secret-copy-operator-system

# Логи
kubectl logs -n secret-copy-operator-system deployment/secret-copy-operator-controller-manager

# Health check
kubectl get --raw /healthz -n secret-copy-operator-system
```

## Удаление

```bash
kubectl delete -k config/default
```
