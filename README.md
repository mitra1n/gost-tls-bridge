# gost-tls-bridge

Кроссплатформенный CLI-лаунчер для **ГОСТ TLS MITM-моста перед Burp Suite**.

Сайты на ГОСТ TLS (сюиты вида `TLS_GOSTR341112_256...`, openssl-имя
`GOST2012-GOST8912-GOST8912`) отвечают обычному Burp/curl `handshake_failure`
(alert 40) на ClientHello. Этот мост делает ГОСТ-рукопожатие за вас, а Burp
видит обычный HTTP в Proxy/Repeater.

> Инструмент для **авторизованного** тестирования безопасности. Используйте
> только на системах, которые вам разрешено проверять.

## Как это работает

```
Burp --(обычный TLS, relay-серт)--> front_listen (stunnel+OpenSSL)
     --plaintext-->                 gost_listen  (ГОСТ-плечо)
     --(ГОСТ TLS)-->                target_host:443
```

Два процесса stunnel:

- **frontend** — обычный `stunnel` + OpenSSL, презентует Burp self-signed
  relay-серт (генерируется автоматически, `crypto/x509`, без openssl).
- **GOST-плечо** — исходящее ГОСТ-рукопожатие. Бэкенд зависит от ОС:
  - **Windows** — `stunnel-msspi` через **CryptoPro CSP** (проверенный путь).
  - **Linux** — `stunnel` собранный с **OpenSSL gost-engine** (best-effort).

## Установка

```
make build            # ./bin/gost-tls-bridge для текущей ОС
make dist             # dist/*.exe (windows) и linux бинарь
```

Или `go build -o gost-tls-bridge .`

### Внешние зависимости (не входят в бинарь)

- **Windows:** `stunnel-msspi.exe` (релиз `stunnel-5.78-msspi-0.84`,
  asset `...-amd64-windows.zip`) + обычный `stunnel.exe` с `libssl-3`/`libcrypto-3`
  (из `stunnel-5.80-win64-installer.exe`, распаковать 7-Zip). CryptoPro CSP.
- **Linux:** `stunnel` + `gost-engine` для OpenSSL.

## Быстрый старт

```
gost-tls-bridge init -target gost.example.ru      # конфиг + relay-серт
gost-tls-bridge start                                 # поднять оба плеча
gost-tls-bridge status                                # процессы + порты
gost-tls-bridge check                                 # end-to-end HTTPS GET
gost-tls-bridge burp                                  # шаги настройки Burp
gost-tls-bridge stop
```

Конфиг и артефакты по умолчанию: `C:\gost\` (Windows) или
`~/.gost-tls-bridge/` (Linux). Другой путь — флаг `-c`.

## Команды

| команда   | что делает                                             |
|-----------|--------------------------------------------------------|
| `init`    | создать workdir, конфиг, relay-серт                    |
| `certgen` | (пере)сгенерировать relay-серт                         |
| `render`  | только отрендерить stunnel-конфиги                     |
| `start`   | отрендерить и поднять оба плеча                        |
| `stop`    | остановить оба плеча                                   |
| `restart` | stop + start                                           |
| `status`  | состояние процессов и портов                           |
| `check`   | сквозная проверка (HTTPS GET через мост)               |
| `burp`    | напечатать шаги настройки Burp                         |

## Настройка Burp (кратко)

1. **Network → DNS**: `target_host → 127.0.0.1` (Enabled). В Burp 2026
   hostname resolution живёт здесь, а не в Network → Connections.
2. **Settings → Network → TLS**: отключить проверку upstream-серта
   (иначе Burp рвёт TLS к нашему self-signed relay).
3. **Proxy → TLS pass-through**: убрать оттуда `target_host`.

`gost-tls-bridge burp` печатает это под ваш конфиг.

## Проверка без Burp

```
curl -k https://127.0.0.1/ -H "Host: gost.example.ru"
```

## Заметки по надёжности

- Мост **не служба** — после ребута/закрытия сессии запускать `start` заново.
- Порт `8080` часто занят — внутренний хоп по умолчанию `18080`.
- `stunnel-msspi` собран **без OpenSSL**: серверную ГОСТ-роль он не умеет и
  PEM-серт не грузит — поэтому фронтенд для Burp только на обычном stunnel.
