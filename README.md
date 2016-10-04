# Ethduck

Building an ephemeral Go (Baduk) webapp, using Go, with a library I built with Go, integrated with Ethereum via [BlockCypher's API](https://www.blockcypher.com/). Right now it's in a deep alpha state, but it works! Allows 2-of-2 multsig games with a wager, where the moves (and some pieces of authorization) are embedded into a smart contract on Ethereum (included in the code here).

# To Install

You must have Go installed. Clone into the repository, then run `go build` in your directory. It will make an executable that will run as the web server. Right now, there is no permanent memory state; however, games can be recovered by their state in the Ethereum blockchain.

# To Do

* Oh man, too much to list, but to start:
* Winner/draw flow
* Betting on individual moves
* UI/UX up the wazoo
