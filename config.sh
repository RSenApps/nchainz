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
    nchainz create LIZ 100000000 5 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    nchainz create NEGAN 100000000 2 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    ;;
  "MAKEMARKET")
    nchainz order 1 LIZ 990 NEGAN 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    nchainz order 1100 NEGAN 1 LIZ 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    nchainz order 1 LIZ 990 NEGAN 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    nchainz order 1005 NEGAN 1 LIZ 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    nchainz order 1 LIZ 850 NEGAN 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    nchainz order 1042 NEGAN 1 LIZ 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    nchainz order 1 LIZ 934 NEGAN 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    nchainz order 1023 NEGAN 1 LIZ 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    nchainz order 1 LIZ 923 NEGAN 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    nchainz order 1092 NEGAN 1 LIZ 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    nchainz order 1 LIZ 929 NEGAN 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    nchainz order 1105 NEGAN 1 LIZ 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    ;;
  "TAKEMARKET")
    nchainz order 1 LIZ 1100 NEGAN 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    nchainz order 990 NEGAN 1 LIZ 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    nchainz order 1 LIZ 1090 NEGAN 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    nchainz order 850 NEGAN 1 LIZ 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    nchainz order 1 LIZ 1010 NEGAN 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    nchainz order 900 NEGAN 1 LIZ 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    nchainz order 1 LIZ 1039 NEGAN 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    nchainz order 905 NEGAN 1 LIZ 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    nchainz order 1 LIZ 1017 NEGAN 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    nchainz order 957 NEGAN 1 LIZ 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    nchainz order 1 LIZ 1063 NEGAN 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    nchainz order 963 NEGAN 1 LIZ 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    nchainz order 1 LIZ 1033 NEGAN 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
    nchainz order 910 NEGAN 1 LIZ 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K
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

