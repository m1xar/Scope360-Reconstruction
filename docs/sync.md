# `Sync` — все модели одним вызовом (v0.3)

## Зачем

До v0.3 каждый публичный метод (`GetBuiltPositions`, `GetOpenPositions`,
`GetBalanceSnapshots`, `GetCurrentBalance`, `GetTransactions`, `GetFundings`)
сам ходил на биржу. Бэкенду, которому нужны все шесть, одни и те же
эндпоинты доставались по 2–4 раза: у OKX `bills-archive` четырежды и
`balance` трижды, у Bybit/Binance леджер и открытые позиции по 2–3 раза, у
MEXC/Orderly/cTrader `GetBalanceSnapshots` заново гонял весь конвейер
закрытых позиций вместе со свечами.

`Sync(days)` грузит сырые данные **один раз** и строит все модели из них.

## Контракт

```go
// pkg/domain/sync.go
type Sync struct {            // крипто: OKX, Binance, Bybit, BloFin, Kraken, MEXC, Hyperliquid, Orderly
    Positions        []Position
    OpenPositions    []OpenPosition
    BalanceSnapshots []UserBalanceSnapshot
    CurrentBalance   float64
    Transactions     []Transaction
    Fundings         []UserFunding
}
type SyncFX struct {          // cTrader
    Positions        []FXPosition
    OpenPositions    []FXOpenPosition
    BalanceSnapshots []UserBalanceSnapshot
    AccountInfo      FXAccountInfo
    Transactions     []Transaction
}
```

Сигнатуры повторяют `GetBuiltPositions` каждой биржи:

| Биржа | Вызов |
|---|---|
| OKX | `okx.Sync(client, creds, baseURL, days)` |
| Binance | `binance.Sync(client, creds, days)` |
| Bybit | `bybit.Sync(client, creds, days)` |
| BloFin | `blofin.Sync(client, creds, days)` |
| Kraken | `kraken.Sync(client, creds, days)` |
| MEXC | `mexc.Sync(client, creds, days)` |
| Hyperliquid | `hyperliquid.Sync(client, endpoint, user, days)` |
| Orderly | `perptools.Sync(client, cfg, days)` |
| cTrader | `ctrader.Sync(client, cfg, days)` → `*domain.SyncFX` |

Каждое поле — ровно то, что вернул бы соответствующий `Get*` с тем же `days`
(те же фильтры по cutoff, та же сортировка). Любая ошибка загрузки или сборки
— `Sync` возвращает `error` целиком (fail-fast, как остальные методы).
`GetCandles`, `GetAuthStatus`, `ValidateWalletSubscription`,
`GetClosedPositionByExactMatch` в `Sync` не входят. Старые методы работают как
прежде, сигнатуры не менялись.

## Как устроено

В каждом `service/reconstructor` появился `dataset.go`:

```go
type Dataset struct { /* сырые ответы биржи */ }
func Load(client, …, cutoff *time.Time, s scope.Scope) (*Dataset, error)
func (d *Dataset) ClosedPositions() ([]domain.Position, error) // + свечи для MAE/MFE
func (d *Dataset) OpenPositions()   ([]domain.OpenPosition, error)
func (d *Dataset) BalanceSnapshots() …
func (d *Dataset) CurrentBalance()   float64
func (d *Dataset) Transactions()     …
func (d *Dataset) Fundings()         …
```

`pkg/reconstruction/scope` — биты `Closed | Open | Balances | Transactions |
Fundings` (`All`). `Load` грузит только датасеты, нужные хоть одному
потребителю из scope, а диапазон каждого берёт как минимум по всем
запросившим (OKX bills: от `min(открытие самой старой закрытой, cutoff)`;
Binance income: от `min(cutoff, открытие самой старой выжившей группы)` и
т.д.). Независимые запросы идут параллельно через
`pkg/reconstruction/parallel.Run`, зависимые (позиции → oldestMs →
ордера/fills/bills) — фазами.

`api.go` каждой биржи: `Sync = Load(scope.All)` + пять билдеров в горутинах;
`GetBuiltPositions = Load(scope.Closed) + ClosedPositions()`,
`GetTransactions = Load(scope.Transactions) + Transactions()` и т.д.
Отдельные методы остались «тонкими» (грузят только своё), логика сборки не
дублируется.

## Что объединяется по биржам

| Биржа | Один раз в `Load(All)` |
|---|---|
| OKX | balance; closed/open positions; instruments по объединению instId; orders+fills от `min(oldest closed, oldest open − 7д)`; **bills всех instType** от `min(oldest closed, cutoff)`, в памяти делятся на SWAP/FUTURES (снапшоты, фандинги `type=8`) и все (транзакции) |
| Binance | account; open positions; symbolConfig; income `type=""` от cutoff (+ одно расширение назад до самой старой группы); **один обход userTrades** на закрытые и открытые (стоп: прошли cutoff **и** все открытые разрешены); orders по объединению fills; конвертация комиссий одним проходом |
| Bybit | account info; open positions; wallet; **один `CollectWeeks`** на закрытые и открытые — его нефильтрованный леджер даёт снапшоты, транзакции и фандинги |
| BloFin | instruments; open positions; один fills-обход (идёт до самой старой открытой); orders от `min(...)`; funding fees (7 дней ретенции — один запрос); equity; transfers от `min(BalanceWindowStart, cutoff)` |
| Kraken | open positions; **`FetchTickers` один раз** вместо N `FetchTicker` (fallback на по-символьный запрос только для отсутствующих); fills-обход; position events; **account-log один раз** от `min(oldest open, cutoff)` → BalanceInit, снапшоты, транзакции, фандинги; accounts (fallback — последний снапшот) |
| MEXC | history positions; contract details; history orders от `min(oldest closed, oldest open)`; funding от `min(oldest closed, cutoff)`; equity; transfers от `min(BalanceWindowStart, cutoff)`; open positions. `GetBalanceSnapshots` больше не гоняет свечи |
| Hyperliquid | `FetchAllFills` (полная история — нужна открытым) → из неё же закрытые эпизоды и снапшоты; historical orders; funding от `min(oldest episode, cutoff)`; portfolio; ledger updates от cutoff |
| Orderly | positions snapshot один раз (open rows, AccountValue, risk-enrich); один trades-обход до самой старой открытой; filled orders; algo orders; funding; asset history от `min(BalanceWindowStart, cutoff)`; mark prices |
| cTrader | session; `LoadHistory(days)` один раз; позиции строятся один раз (с MAE/MFE), снапшоты — из них; reconcile + spots; trader + assets; cash flow. Один TLS-стрим → сырьё грузится последовательно, билдеры параллельно |

Обход fills для групп 2 (Binance, Bybit, BloFin, Kraken, Orderly) при
`Closed|Open` останавливается только когда прошёл cutoff **и** дошёл до
открытия каждой открытой позиции (`Resolved()` у walker'а или явный «reach»
до `OpenTime` от биржи) — иначе открытая позиция без сделок в окне осталась
бы без ордеров.

## Проверка

Живой харнес `tmp/sync_smoke` (gitignored): для каждого аккаунта `Sync(days)`
и шесть `Get*` параллельно на том же клиенте, сравнение полей, время и число
HTTP-запросов. Регрессия против предыдущего тега: `tmp/multi_smoke` дампы +
`tmp/compare.py`.
