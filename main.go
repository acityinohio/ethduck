package main

import (
	"bufio"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"strings"
	"text/template"

	"./bcyeth"
	"./bcytoken"

	"github.com/acityinohio/baduk"
)

type Game struct {
	ContractAddr string
	Confirmed    bool
	BlackTurn    bool
	ApprovalLock bool
	ProposedMove string
	State        baduk.Board
}

var templates = template.Must(template.ParseGlob("templates/*"))
var bcy bcyeth.API

func init() {
	bcy = bcyeth.API{bcytoken.Token, "eth", "main"}
}

func main() {
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/games/", gameHandler)
	http.HandleFunc("/new/", newGameHandler)
	http.HandleFunc("/authorize/", authorizeHandler)
	http.ListenAndServe(":8080", nil)
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	err := templates.ExecuteTemplate(w, "index.html", "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func newGameHandler(w http.ResponseWriter, r *http.Request) {
	f := r.FormValue
	var err error
	//Initialize Board
	size, err := strconv.Atoi(f("size"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	wager := new(big.Int)
	wager.SetString(f("wager"), 10)
	blackPriv := f("blackPriv")
	whiteAddr := f("whiteAddr")
	//Generate New EthDuck Contract on Ethereum
	contractAddr, err := publishEthDuck(blackPriv, whiteAddr, size, *wager)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, "Your contract address is %s , please wait for it to confirm before playing", contractAddr)
	return
}

func gameHandler(w http.ResponseWriter, r *http.Request) {
	contractAddr := r.URL.Path[len("/games/"):]
	gameBoard, err := remakeGame(contractAddr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if r.Method == "POST" {
		moveHandler(w, r, gameBoard)
		return
	}
	type gameTemp struct {
		Game      Game
		PrettySVG string
	}
	necessary := gameTemp{gameBoard, gameBoard.State.PrettySVG()}
	err = templates.ExecuteTemplate(w, "game.html", necessary)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func remakeGame(contractAddr string) (game Game, err error) {
	game.ContractAddr = contractAddr
	game.Confirmed, err = getConfirmed(contractAddr)
	if err != nil {
		return
	}
	game.BlackTurn, err = getBlackTurn(contractAddr)
	if err != nil {
		return
	}
	game.ApprovalLock, err = getApprovalLock(contractAddr)
	if err != nil {
		return
	}
	if game.ApprovalLock {
		x, y, color := getProposedMove(contractAddr)
		if color == 1 {
			game.ProposedMove = "black-"
		} else {
			game.ProposedMove = "white-"
		}
		game.ProposedMove += strconv.Itoa(x) + "-" + strconv.Itoa(y)
	}
	size, err := getSize(contractAddr)
	if err != nil {
		return
	}
	game.State.Init(size)
	numMoves, err := getNumMoves(contractAddr)
	if err != nil {
		return
	}
	for move := 0; move < numMoves; move++ {
		x, y, color := getMove(contractAddr, move)
		if color == 1 {
			err = game.State.SetB(x, y)
		} else if color == 2 {
			err = game.State.SetW(x, y)
		}
		if err != nil {
			return
		}
	}
	return
}

func moveHandler(w http.ResponseWriter, r *http.Request, gameBoard Game) {
	//Get move, send transaction
	f := r.FormValue
	rawmove := strings.Split(f("orig-message"), "-")
	private := f("private")
	if gameBoard.BlackTurn && rawmove[0] != "black" {
		http.Error(w, "Not black's turn", http.StatusInternalServerError)
		return
	}
	if !gameBoard.BlackTurn && rawmove[0] != "white" {
		http.Error(w, "Not white's turn", http.StatusInternalServerError)
		return
	}
	x, _ := strconv.Atoi(rawmove[1])
	y, _ := strconv.Atoi(rawmove[2])
	err := proposeMove(gameBoard.ContractAddr, private, x, y)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	http.Redirect(w, r, "/authorize/"+gameBoard.ContractAddr, http.StatusFound)
	return
}

func authorizeHandler(w http.ResponseWriter, r *http.Request) {
	contractAddr := r.URL.Path[len("/authorize/"):]
	if r.Method == "POST" {
		f := r.FormValue
		private := f("private")
		approve, _ := strconv.ParseBool(f("approve"))
		err := authorizeMove(contractAddr, private, approve)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/game/"+contractAddr, http.StatusFound)
	} else {
		var message string
		x, y, color := getProposedMove(contractAddr)
		if color == 1 {
			message = "Black "
		} else {
			message = "White "
		}
		message += "wants to move on" + strconv.Itoa(x) + " , " + strconv.Itoa(y) + "."
		err := templates.ExecuteTemplate(w, "authorize.html", message)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		return
	}
}

//contract helpers
func publishEthDuck(blackPriv string, whiteAddr string, size int, wager big.Int) (contractAddr string, err error) {
	contract := bcyeth.Contract{
		Private:  blackPriv,
		Solidity: importSol(),
		Publish:  []string{"EthDuck"},
		Params:   []interface{}{size, whiteAddr},
		Value:    wager,
		GasLimit: 1400000,
	}
	result, err := bcy.CreateContract(contract)
	if err != nil {
		return
	}
	contractAddr = result[0].Address
	return
}

func importSol() (sol string) {
	file, err := os.Open("./ethduck.sol")
	if err != nil {
		panic(err)
	}
	defer file.Close()
	//make solidity one line without tabs for BlockCypher API
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		sol += strings.Replace(scanner.Text(), "\t", " ", -1) + "\n"
	}
	sol = strings.TrimSpace(sol)
	return
}

//make moves
func proposeMove(contractAddr string, private string, x int, y int) (err error) {
	_, err = bcy.CallContract(bcyeth.Contract{Private: private, Params: []interface{}{x, y}, GasLimit: 200000}, contractAddr, "proposeMove")
	return
}

func authorizeMove(contractAddr string, private string, approve bool) (err error) {
	_, err = bcy.CallContract(bcyeth.Contract{Private: private, Params: []interface{}{approve}, GasLimit: 200000}, contractAddr, "authorizeMove")
	return
}

//constant contract methods
func getConfirmed(contractAddr string) (confirmed bool, err error) {
	result, err := bcy.CallContract(bcyeth.Contract{Private: "c025000000000000000000000000000000000000000000000000000000000000"}, contractAddr, "confirmed")
	if err != nil {
		return
	}
	confirmed = result.Results[0].(bool)
	return
}

func getBlackTurn(contractAddr string) (blackTurn bool, err error) {
	result, err := bcy.CallContract(bcyeth.Contract{Private: "c025000000000000000000000000000000000000000000000000000000000000"}, contractAddr, "blackTurn")
	if err != nil {
		return
	}
	blackTurn = result.Results[0].(bool)
	return
}

func getApprovalLock(contractAddr string) (approvalLock bool, err error) {
	result, err := bcy.CallContract(bcyeth.Contract{Private: "c025000000000000000000000000000000000000000000000000000000000000"}, contractAddr, "approvalLock")
	if err != nil {
		return
	}
	approvalLock = result.Results[0].(bool)
	return
}

func getSize(contractAddr string) (size int, err error) {
	result, err := bcy.CallContract(bcyeth.Contract{Private: "c025000000000000000000000000000000000000000000000000000000000000"}, contractAddr, "size")
	if err != nil {
		return
	}
	size = result.Results[0].(int)
	return
}

func getNumMoves(contractAddr string) (numMoves int, err error) {
	result, err := bcy.CallContract(bcyeth.Contract{Private: "c025000000000000000000000000000000000000000000000000000000000000"}, contractAddr, "getNumMoves")
	if err != nil {
		return
	}
	numMoves = result.Results[0].(int)
	return
}

func getMove(contractAddr string, move int) (x, y, color int) {
	result, err := bcy.CallContract(bcyeth.Contract{Private: "c025000000000000000000000000000000000000000000000000000000000000", Params: []interface{}{move}}, contractAddr, "getMove")
	if err != nil {
		return
	}
	x, y, color = result.Results[0].(int), result.Results[1].(int), result.Results[2].(int)
	return
}

func getProposedMove(contractAddr string) (x, y, color int) {
	result, err := bcy.CallContract(bcyeth.Contract{Private: "c025000000000000000000000000000000000000000000000000000000000000"}, contractAddr, "proposed")
	if err != nil {
		return
	}
	x, y, color = result.Results[0].(int), result.Results[1].(int), result.Results[2].(int)
	return
}
