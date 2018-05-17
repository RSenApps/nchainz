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
    nchainz create NEGAN 100000000 5 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    nchainz create LIZ 100000000 2 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    ;;
  "MAKEMARKET")
    nchainz order 1 NEGAN 990 LIZ 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    nchainz order 1100 LIZ 1 NEGAN 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    nchainz order 1 NEGAN 990 LIZ 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    nchainz order 1005 LIZ 1 NEGAN 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    nchainz order 1 NEGAN 850 LIZ 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    nchainz order 1042 LIZ 1 NEGAN 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    nchainz order 1 NEGAN 934 LIZ 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    nchainz order 1023 LIZ 1 NEGAN 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    nchainz order 1 NEGAN 923 LIZ 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    nchainz order 1092 LIZ 1 NEGAN 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    nchainz order 1 NEGAN 929 LIZ 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    nchainz order 1105 LIZ 1 NEGAN 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    ;;
  "TAKEMARKET")
    nchainz order 1 NEGAN 1100 LIZ 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    nchainz order 990 LIZ 1 NEGAN 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    nchainz order 1 NEGAN 1090 LIZ 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    nchainz order 850 LIZ 1 NEGAN 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    nchainz order 1 NEGAN 1010 LIZ 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    nchainz order 900 LIZ 1 NEGAN 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    nchainz order 1 NEGAN 1039 LIZ 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    nchainz order 905 LIZ 1 NEGAN 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    nchainz order 1 NEGAN 1017 LIZ 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    nchainz order 957 LIZ 1 NEGAN 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    nchainz order 1 NEGAN 1063 LIZ 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    nchainz order 963 LIZ 1 NEGAN 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    nchainz order 1 NEGAN 1033 LIZ 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    nchainz order 910 LIZ 1 NEGAN 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
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

