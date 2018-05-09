case $1 in
  "RESET")
    rm -rf db
    mkdir db
    go install
    ;;
  "STOP")    
    pkill -f "dex"
    ;;
  "MC")
    nchainz create ETH 100000000 5 vit
    nchainz create USD 100000000 2 sam
    ;;
  "MAINNET")
    printf "35.172.27.110:5000\n35.174.80.163:5000\n52.87.50.12:5000" > seeds.txt
    ;;
  "LOCALNET")
    printf "localhost:4000\nlocalhost:4001\nlocalhost:4002" > seeds.txt
    ;;
  *)
    echo "Command not found: try again"
    ;;
esac

