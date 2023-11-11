# Introduction

This is a Go application for real-time detection of triangular arbitrage opportunities in cryptocurrency markets.

### Features

* Efficient Real-Time crypto pairs tracking
* Compatible with multiple crypto exchanges
* Precision calculation with fees included
* Notify slack channel when opportunities show up

# Run

```
make run
```

# Deployment

### First time deployment

```
make first_time
```

log in EC2

```
cd /home/ec2-user/app/
sudo cp crypto-triangular-arbitrage-watch.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl start crypto-triangular-arbitrage-watch
sudo systemctl status crypto-triangular-arbitrage-watch
```

### Deploy

1) Copy `config.yml` to `prod-config.yml`

2) Change `ENV` to `prod` in `prod-config.yml`

3) Run

```
make deploy
```

# Debug mode

`config.yml`:

```
DEBUG_PRINT_MESSAGE: true
DEBUG_PRINT_MOST_PROFIT: true
```

# Manual test

Run (optional, it's just for receiving order status)

    make run

Buy BTC with $10 USDT

    make buy QTY=10

Sell BTC with qty=0.000294

    make sell QTY=0.000294

# TODO

* profit > 0.001
    * compare all price * qty and choose the lowest money to put in as the first trade
* graceful shutdown
* mysql to store the process of trade
* Add unit test
* ISSUE: sell response `"cumExecQty":"0.000298"` but it's `0.00029772` on bybit dashboard
