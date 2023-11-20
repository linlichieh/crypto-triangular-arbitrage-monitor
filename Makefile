# Please see README for details
first_time:
	rsync -av -e ssh deployment/crypto-triangular-arbitrage-watch.service tri:/home/ec2-user/app/
deploy:
	env GOOS=linux GOARCH=amd64 go build
	rsync -av -e ssh crypto-triangular-arbitrage-watch tri:/home/ec2-user/app/
	rsync -av -e ssh prod-config.yml tri:/home/ec2-user/app/config.yml
	rsync -av -e ssh prod-symbol_combinations.json tri:/home/ec2-user/app/symbol_combinations.json
	rsync -av -e ssh prod-symbol_instruments.json tri:/home/ec2-user/app/symbol_instruments.json
	ssh -t tri "sudo systemctl restart crypto-triangular-arbitrage-watch"
run:
	go build
	./crypto-triangular-arbitrage-watch
buy:
	@$(if $(sym),\
		go run manual_tests/order.go --action="Buy" --qty=$(qty) --sym=$(sym),\
		go run manual_tests/order.go --action="Buy" --qty=$(qty))
sell:
	@$(if $(sym),\
		go run manual_tests/order.go --action="Sell" --qty=$(qty) --sym=$(sym),\
		go run manual_tests/order.go --action="Sell" --qty=$(qty))
instrument:
	@$(if $(sym),\
		go run manual_tests/order.go --action="instrument" --sym=$(sym),\
		go run manual_tests/order.go --action="instrument")
generate_instruments:
	go run manual_tests/order.go --action="generate_instruments"
all_symbols:
	go run manual_tests/order.go --action="all_symbols"
trii:
	go run manual_tests/order.go --action="trii" --qty=$(qty)
order_history:
	@$(if $(limit),\
        go run manual_tests/order.go --action="order_history" --limit=$(limit),\
        go run manual_tests/order.go --action="order_history")

