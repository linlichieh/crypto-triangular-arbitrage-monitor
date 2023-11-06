# Introduction

This is a Go application for real-time detection of triangular arbitrage opportunities in cryptocurrency markets.

### Features

* Efficient Real-Time crypto pairs tracking
* Compatible with multiple crypto exchanges
* Precision calculation with fees included
* Notify slack channel when opportunities show up

# Run

    make run

# Deployment

### First time deployment

    make first_time

log in EC2

    cd /home/ec2-user/app/
    sudo cp crypto-triangular-arbitrage-watch.service /etc/systemd/system/
    sudo systemctl daemon-reload
    sudo systemctl start crypto-triangular-arbitrage-watch
    sudo systemctl status crypto-triangular-arbitrage-watch

### Deploy

1) Copy `config.yml` to `prod-config.yml`

2) Change `ENV` to `prod` in `prod-config.yml`

3) Run

    make deploy
