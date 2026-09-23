# Символы: `Symbol`, `NormalizeSymbol`, `DenormalizeSymbol` (v0.4)

## Зачем

Модели отдают нормализованный `Pair` (`BTCUSDT`, `XBTUSD`, `kPEPEUSDC`,
`EURUSD`), а `GetCandles` каждой биржи ждёт её родной идентификатор
(`BTC-USDT-SWAP`, `PF_XBTUSD`, `BTC` у Hyperliquid, `BTC_USDT` у MEXC,
`PERP_BTC_USDC` у Orderly, имя символа у cTrader). Нормализация местами
необратима: OKX режет `-SWAP` и дату экспирации, Kraken — префикс
`PF_`/`PI_`/`FI_`, Hyperliquid — dex-префикс `xyz:`. До v0.4 по позиции из
`Sync` свечи было не получить.

## Что добавилось

### Поле `Symbol` в моделях

`Position`, `OpenPosition`, `UserFunding`, `FXPosition`, `FXOpenPosition`
получили `Symbol string` — оригинальный идентификатор биржи, из которого
построен `Pair`:

| Биржа | `Symbol` | `Pair` |
|---|---|---|
| OKX | `BTC-USDT-SWAP`, `BTC-USD-260925` | `BTCUSDT`, `BTCUSD` |
| Binance / Bybit | `BTCUSDT` | `BTCUSDT` |
| BloFin | `BTC-USDT` | `BTCUSDT` |
| Kraken | `PF_XBTUSD` | `XBTUSD` |
| MEXC | `BTC_USDT` | `BTCUSDT` |
| Hyperliquid | `BTC`, `kPEPE`, `xyz:BTC` | `BTCUSDC`, `kPEPEUSDC`, `BTCUSDC` |
| Orderly | `PERP_BTC_USDC` | `BTCUSDC` |
| cTrader | `EUR/USD` | `EURUSD` |

Это точный оригинал даже для датированных контрактов — для свечей по своим
позициям бэкенду достаточно хранить `Symbol` рядом с `Pair`.

### `NormalizeSymbol` / `DenormalizeSymbol` в каждом `api.go`

```go
okx.NormalizeSymbol("BTC-USDT-SWAP")            // "BTCUSDT"
okx.DenormalizeSymbol(client, baseURL, "BTCUSDT") // "BTC-USDT-SWAP", nil

binance.DenormalizeSymbol(client, pair)          // Binance, Bybit, BloFin, Kraken, MEXC
hyperliquid.DenormalizeSymbol(client, endpoint, pair)
perptools.DenormalizeSymbol(client, cfg, pair)   // Orderly
ctrader.DenormalizeSymbol(client, cfg, pair)     // имя символа брокера, например "EUR/USD"
```

`DenormalizeSymbol` принимает всё, что похоже на символ: нормализованный
`Pair`, оригинал (возвращается как есть), любой регистр и разделители
(`btc/usdt`, `BTC-USDT`). `client == nil` → публичный клиент без ключей.

`GetCandles` и `GetClosedPositionByExactMatch` теперь принимают и
нормализованный, и оригинальный символ: `GetCandles` первым делом
вызывает `DenormalizeSymbol` (если резолв не удался — например, контракт
делистнут и его нет ни в seed, ни в живом списке, — символ уходит на биржу
как есть), `ExactMatch` сравнивает пары по общему ключу
(верхний регистр, только буквы и цифры). У Orderly `GetCandles` по-прежнему
принимает и голый `coin` (`BTC` → `PERP_BTC_USDC`).

## Как устроено

`pkg/reconstruction/symbols` — `Registry`: словарь `ключ(Pair) → Symbol`
плюс множество известных оригиналов. `pkg/<биржа>/service/symbols`:

* `seed.json` — снимок универсума биржи (`[{"symbol","pair"}]` в порядке
  приоритета), вшит через `go:embed`; при старте процесса словарь готов
  без единого запроса.
* `Universe(client, …)` — живой список инструментов с биржи:
  OKX `/public/instruments` (SWAP, затем FUTURES), Binance `exchangeInfo`,
  Bybit `instruments-info` (linear: торгуемые, затем `status=Closed` —
  делистнутые, свечи за период торгов у них есть), BloFin `/market/instruments`, Kraken
  `/tickers` (symbol + pair), MEXC `/contract/detail`, Hyperliquid
  `{"type":"meta"}`, Orderly `/v1/public/info`. Все публичные.
* `Denormalize` = `Registry.Resolve(pair, Universe)`.

`Resolve`: оригинал → как есть; ключ найден → символ; промах → если с
последнего обновления прошло больше 5 минут, один запрос `Universe`,
словарь пересобирается (свежий список первым, затем записи seed, которых
в нём уже нет — делистнутые остаются), повтор поиска; иначе ошибка
`symbol "X" not found`. Обновление под мьютексом, конкурентные промахи
ждут один запрос.

### Коллизии

Один `Pair` могут давать несколько контрактов: OKX `BTC-USD-SWAP` и
`BTC-USD-260925`, Kraken `PF_ETHUSD` / `PI_ETHUSD` / `FI_ETHUSD_260925`.
Порядок в универсуме — перп первым (OKX SWAP → FUTURES по экспирации;
Kraken `PF_` → `PI_` → `FI_` → `FF_`; Binance `PERPETUAL` → по
`deliveryDate`; Bybit `LinearPerpetual` → по `deliveryTime`), первый
выигрывает. Для датированной позиции `DenormalizeSymbol(Pair)` вернёт перп —
точный контракт лежит в `Symbol` модели.

cTrader словаря нет: список символов per-broker и под сессией, поэтому
`DenormalizeSymbol` каждый раз берёт `ProtoOASymbolsListReq` (как и
`GetCandles` до v0.4).

## Перегенерация seed

```
go run ./cmd/symbolseed
```

Обходит публичные эндпоинты всех бирж, переписывает `seed.json` и печатает
размер универсума и коллизии. Запускать перед релизом; между релизами новые
листинги подхватываются фолбеком на бирже.

## Проверка

Живой харнес `tmp/symbol_smoke` (gitignored): для каждого аккаунта
`Sync(days)`, уникальные `Pair` из позиций/открытых/фандингов,
`DenormalizeSymbol(Pair) == Symbol`, `GetCandles` с оригиналом и с `Pair`
дают одинаковый результат, число HTTP-запросов на резолв (по seed — 0), один
заведомый промах — ровно один поход за универсумом.
