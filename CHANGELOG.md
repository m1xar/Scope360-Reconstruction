# Changelog

## v0.4.4

- FX (Match-Trader, DXtrade, cTrader): `FXPosition.ContractSize` и
  `FXOpenPosition.ContractSize *float64` — единиц базового актива в лоте
  (не деньги за пункт); `nil`, если площадка не отдала.
- `Dataset.ContractSizes()` и `GetContractSizes(client, cfg)` в каждом
  FX-коннекторе → `[]domain.FXSymbolSpec` по всем символам счёта для бэкфила.
- Match-Trader: `Load` дополнительно запрашивает `effective-instruments`
  (ошибка не фатальна).
- Поведение `Amount` и остальных полей не менялось. Подробно:
  [docs/contract-size.md](docs/contract-size.md).
