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
  *)
    echo "Command not found: try again"
    ;;
esac

