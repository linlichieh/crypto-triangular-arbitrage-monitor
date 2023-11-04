first_time:
	rsync -av -e ssh deployment/crypto-triangular-arbitrage-watch.service tri:/home/ec2-user/app/
deploy:
	env GOOS=linux GOARCH=amd64 go build
	rsync -av -e ssh crypto-triangular-arbitrage-watch tri:/home/ec2-user/app/
	rsync -av -e ssh prod-config.yml tri:/home/ec2-user/app/config.yml
	rsync -av -e ssh symbol_combinations.json tri:/home/ec2-user/app/
	ssh -t tri "sudo systemctl restart crypto-triangular-arbitrage-watch"
