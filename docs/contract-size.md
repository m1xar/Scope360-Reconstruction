# Contract size в FX-коннекторах (v0.4.4)

## Зачем

Match-Trader, DXtrade и cTrader отдают размер контракта (сколько единиц
актива в одном лоте), а DXtrade и cTrader уже использовали его, чтобы
перевести объём в лоты. Наружу он не выходил: бэкенду нечем было показать
объём в юнитах или понять, в чём записан `Amount`.

## Что добавилось

### Поле `ContractSize` в `FXPosition` и `FXOpenPosition`

```go
ContractSize *float64 // единиц базового актива в 1 лоте; nil = площадка не отдала
```

| Площадка | Откуда | Пример |
|---|---|---|
| Match-Trader | `effective-instruments` → `contractSize` | EURUSD 100000 |
| DXtrade | `/instruments/query` → `lotSize` | EURUSD 100000 |
| cTrader | `ProtoOASymbol.lotSize / 100` (lotSize в центах юнита, как все объёмы Open API) | lotSize 10000000 → 100000 |

`nil` — площадка не отдала значение (инструмента нет в списке, `0`,
у Match-Trader не удался запрос инструментов, у DXtrade 404 на список).
Это важно: без размера DXtrade кладёт в `Amount` юниты, а cTrader
`volume/100` — по `nil` такие позиции теперь видно. Поведение `Amount` не
менялось.

### `Dataset.ContractSizes()` и `GetContractSizes`

```go
specs, err := matchtrader.GetContractSizes(client, cfg) // так же dxtrade.*, ctrader.*
// или на уже загруженном датасете:
specs, err := d.ContractSizes() // []domain.FXSymbolSpec{Symbol, Pair, ContractSize, Source}
```

Спецификация по всем символам счёта без загрузки истории — для бэкфила
старых позиций: `UPDATE ... SET contract_size = :size WHERE pair = :pair AND
contract_size IS NULL`. `Pair` совпадает с `FXPosition.Pair`. Если у брокера
два символа с одной парой (`EURUSD` и `EURUSD.mini`), матчить по `Symbol`.
Значение текущее, а не то, что действовало при открытии старой позиции.

Сетевые запросы: Match-Trader — +1 GET `effective-instruments` на синк
(ответ кэшируется в клиенте, ошибка синк не роняет; `ContractSizes()` при
`Load(..., 0)` запрашивает его сам). DXtrade — ничего нового. cTrader —
`ContractSizes()` один раз догружает `SymbolByIdReq` для неторгованных
символов пачками по 100.

## Единицы — не путать

Contract size — **единицы актива в лоте, а не деньги за пункт**. Для пар, где
валюта прибыли не совпадает с валютой депозита (USDCAD, GER30 на USD-счёте),
`Amount × ContractSize × Δцены` не равно PnL без конвертации. Пересчитывать
по нему PnL или MAE нельзя.

## Совместимость

Новое поле в конце структур и новые методы — потребители v0.4.3 с
именованными литералами собираются без изменений и получают `nil`.
