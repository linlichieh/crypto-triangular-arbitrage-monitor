# Please see README for details
first_time:
	rsync -av -e ssh deployment/crypto-triangular-arbitrage-watch.service tri:/home/ec2-user/app/
deploy:
	env GOOS=linux GOARCH=amd64 go build
	rsync -av -e ssh crypto-triangular-arbitrage-watch tri:/home/ec2-user/app/
	rsync -av -e ssh prod-config.yml tri:/home/ec2-user/app/config.yml
	rsync -av -e ssh prod-symbol_combinations.json tri:/home/ec2-user/app/symbol_combinations.json
	ssh -t tri "sudo systemctl restart crypto-triangular-arbitrage-watch"
run:
	go build
	./crypto-triangular-arbitrage-watch
buy:
	go run manual_tests/order.go --action="Buy" --qty=$(qty)
sell:
	go run manual_tests/order.go --action="Sell" --qty=$(qty)
trii:
	go run manual_tests/order.go --action="trii"
