package main

import (
	"bufio"
	"encoding/json"
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
	Draw         bool
	Winner       int
	BlackScore   int
	WhiteScore   int
	ProposedMove string
	State        baduk.Board
}

var templates = template.Must(template.ParseGlob("templates/*"))
var bcy bcyeth.API

func init() {
	bcy = bcyeth.API{bcytoken.Token}
}

func main() {
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/games/", gameHandler)
	http.HandleFunc("/new/", newGameHandler)
	http.HandleFunc("/confirm/", confirmGameHandler)
	http.HandleFunc("/propose/win/", proposeWinHandler)
	http.HandleFunc("/propose/draw/", proposeDrawHandler)
	http.HandleFunc("/auth/move/", authorizeMoveHandler)
	http.HandleFunc("/auth/win/", authorizeWinHandler)
	http.HandleFunc("/auth/draw/", authorizeDrawHandler)
	http.ListenAndServe(":80", nil)
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

func confirmGameHandler(w http.ResponseWriter, r *http.Request) {
	contractAddr := r.URL.Path[len("/confirm/"):]
	if r.Method == "POST" {
		f := r.FormValue
		private := f("private")
		approve, _ := strconv.ParseBool(f("approve"))
		if approve != true {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
		addr, err := bcy.GetAddrBal(contractAddr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		err = confirmNewGame(contractAddr, private, addr.Balance)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/games/"+contractAddr, http.StatusFound)
		return
	} else {
		confirmed, err := getConfirmed(contractAddr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		addr, err := bcy.GetAddrBal(contractAddr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		var message string
		if confirmed {
			message = "This game is already confirmed, you don't need to send money to this contract."
		} else {
			message = "You need to confirm your game. Please enter your private key. The wei-ger is " + addr.Balance.String() + "."
		}
		data := struct {
			Message string
			Post    string
		}{
			message,
			"/confirm/" + contractAddr,
		}
		err = templates.ExecuteTemplate(w, "authorize.html", data)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		return
	}
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
		Game
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
	game.Draw, err = getDraw(contractAddr)
	if err != nil {
		return
	}
	game.Winner, err = getWinner(contractAddr)
	if err != nil {
		return
	}
	if game.ApprovalLock && !game.Draw && game.Winner == 0 {
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
	game.BlackScore, game.WhiteScore = game.State.Score()
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
	http.Redirect(w, r, "/games/"+gameBoard.ContractAddr, http.StatusFound)
	return
}

func proposeWinHandler(w http.ResponseWriter, r *http.Request) {
	contractAddr := r.URL.Path[len("/propose/win/"):]
	if r.Method == "POST" {
		f := r.FormValue
		private := f("private")
		approve, _ := strconv.ParseBool(f("approve"))
		if approve != true {
			http.Redirect(w, r, "/games/"+contractAddr, http.StatusFound)
			return
		}
		err := proposeWinner(contractAddr, private)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/games/"+contractAddr, http.StatusFound)
		return
	} else {
		winner, err := getWinner(contractAddr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		var message string
		if winner == 1 {
			message = "Black stone winner already proposed! Needs confirmation."
		} else if winner == 2 {
			message = "White stone winner already proposed! Needs confirmation."
		} else {
			message = "Propose yourself winner! Make sure it's your turn, then enter your private key here."
		}
		data := struct {
			Message string
			Post    string
		}{
			message,
			"/propose/win/" + contractAddr,
		}
		err = templates.ExecuteTemplate(w, "authorize.html", data)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		return
	}
}

func proposeDrawHandler(w http.ResponseWriter, r *http.Request) {
	contractAddr := r.URL.Path[len("/propose/draw/"):]
	if r.Method == "POST" {
		f := r.FormValue
		private := f("private")
		approve, _ := strconv.ParseBool(f("approve"))
		if approve != true {
			http.Redirect(w, r, "/games/"+contractAddr, http.StatusFound)
			return
		}
		err := proposeDraw(contractAddr, private)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/games/"+contractAddr, http.StatusFound)
		return
	} else {
		draw, err := getDraw(contractAddr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		var message string
		if draw {
			message = "Draw already proposed! Need confirmation."
		} else {
			message = "Propose a draw! Make sure it's your turn, then enter your private key here."
		}
		data := struct {
			Message string
			Post    string
		}{
			message,
			"/propose/draw/" + contractAddr,
		}
		err = templates.ExecuteTemplate(w, "authorize.html", data)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		return
	}
}

func authorizeMoveHandler(w http.ResponseWriter, r *http.Request) {
	contractAddr := r.URL.Path[len("/auth/move/"):]
	if r.Method == "POST" {
		f := r.FormValue
		private := f("private")
		approve, _ := strconv.ParseBool(f("approve"))
		err := authorizeMove(contractAddr, private, approve)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/games/"+contractAddr, http.StatusFound)
	} else {
		var message string
		x, y, color := getProposedMove(contractAddr)
		if color == 1 {
			message = "Black "
		} else {
			message = "White "
		}
		message += "wants to move on " + strconv.Itoa(x) + ", " + strconv.Itoa(y) + "."
		data := struct {
			Message string
			Post    string
		}{
			message,
			"/auth/move/" + contractAddr,
		}
		err := templates.ExecuteTemplate(w, "authorize.html", data)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		return
	}
}

func authorizeWinHandler(w http.ResponseWriter, r *http.Request) {
	contractAddr := r.URL.Path[len("/auth/win/"):]
	if r.Method == "POST" {
		f := r.FormValue
		private := f("private")
		approve, _ := strconv.ParseBool(f("approve"))
		err := authorizeWinner(contractAddr, private, approve)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/games/"+contractAddr, http.StatusFound)
		return
	} else {
		winner, err := getWinner(contractAddr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		var message string
		if winner == 1 {
			message = "Black stone proposed that they won!"
		} else if winner == 2 {
			message = "White stone proposed that they won!"
		} else {
			http.Redirect(w, r, "/games/"+contractAddr, http.StatusFound)
		}
		data := struct {
			Message string
			Post    string
		}{
			message,
			"/auth/win/" + contractAddr,
		}
		err = templates.ExecuteTemplate(w, "authorize.html", data)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		return
	}
}

func authorizeDrawHandler(w http.ResponseWriter, r *http.Request) {
	contractAddr := r.URL.Path[len("/auth/draw/"):]
	if r.Method == "POST" {
		f := r.FormValue
		private := f("private")
		approve, _ := strconv.ParseBool(f("approve"))
		err := authorizeDraw(contractAddr, private, approve)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/games/"+contractAddr, http.StatusFound)
		return
	} else {
		draw, err := getDraw(contractAddr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		var message string
		if draw {
			message = "Draw proposed!"
		} else {
			http.Redirect(w, r, "/games/"+contractAddr, http.StatusFound)
		}
		data := struct {
			Message string
			Post    string
		}{
			message,
			"/auth/draw/" + contractAddr,
		}
		err = templates.ExecuteTemplate(w, "authorize.html", data)
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
	//convert solidity to string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		sol += strings.Replace(scanner.Text(), "\t", " ", -1) + "\n"
	}
	sol = strings.TrimSpace(sol)
	return
}

//confirm game, add wager
func confirmNewGame(contractAddr string, private string, value big.Int) (err error) {
	_, err = bcy.CallContract(bcyeth.Contract{Private: private, GasLimit: 100000, Value: value}, contractAddr, "confirmNewGame")
	return
}

//make moves
func proposeMove(contractAddr string, private string, x int, y int) (err error) {
	_, err = bcy.CallContract(bcyeth.Contract{Private: private, Params: []interface{}{x, y}, GasLimit: 100000}, contractAddr, "proposeMove")
	return
}

func authorizeMove(contractAddr string, private string, approve bool) (err error) {
	_, err = bcy.CallContract(bcyeth.Contract{Private: private, Params: []interface{}{approve}, GasLimit: 200000}, contractAddr, "authorizeMove")
	return
}

func proposeDraw(contractAddr string, private string) (err error) {
	_, err = bcy.CallContract(bcyeth.Contract{Private: private, GasLimit: 100000}, contractAddr, "proposeDraw")
	return
}

func authorizeDraw(contractAddr string, private string, approve bool) (err error) {
	_, err = bcy.CallContract(bcyeth.Contract{Private: private, Params: []interface{}{approve}, GasLimit: 200000}, contractAddr, "authorizeDraw")
	return
}

func proposeWinner(contractAddr string, private string) (err error) {
	_, err = bcy.CallContract(bcyeth.Contract{Private: private, GasLimit: 100000}, contractAddr, "proposeWinner")
	return
}

func authorizeWinner(contractAddr string, private string, approve bool) (err error) {
	_, err = bcy.CallContract(bcyeth.Contract{Private: private, Params: []interface{}{approve}, GasLimit: 200000}, contractAddr, "authorizeWinner")
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

func getWinner(contractAddr string) (winner int, err error) {
	result, err := bcy.CallContract(bcyeth.Contract{Private: "c025000000000000000000000000000000000000000000000000000000000000"}, contractAddr, "winner")
	if err != nil {
		return
	}
	num, err := result.Results[0].(json.Number).Int64()
	if err != nil {
		return
	}
	winner = int(num)
	return
}

func getDraw(contractAddr string) (draw bool, err error) {
	result, err := bcy.CallContract(bcyeth.Contract{Private: "c025000000000000000000000000000000000000000000000000000000000000"}, contractAddr, "draw")
	if err != nil {
		return
	}
	draw = result.Results[0].(bool)
	return
}

func getSize(contractAddr string) (size int, err error) {
	result, err := bcy.CallContract(bcyeth.Contract{Private: "c025000000000000000000000000000000000000000000000000000000000000"}, contractAddr, "size")
	if err != nil {
		return
	}
	num, err := result.Results[0].(json.Number).Int64()
	if err != nil {
		return
	}
	size = int(num)
	return
}

func getNumMoves(contractAddr string) (numMoves int, err error) {
	result, err := bcy.CallContract(bcyeth.Contract{Private: "c025000000000000000000000000000000000000000000000000000000000000"}, contractAddr, "getNumMoves")
	if err != nil {
		return
	}
	num, err := result.Results[0].(json.Number).Int64()
	if err != nil {
		return
	}
	numMoves = int(num)
	return
}

func getMove(contractAddr string, move int) (x, y, color int) {
	result, err := bcy.CallContract(bcyeth.Contract{Private: "c025000000000000000000000000000000000000000000000000000000000000", Params: []interface{}{move}}, contractAddr, "getMove")
	if err != nil {
		return
	}
	xNum, err := result.Results[0].(json.Number).Int64()
	if err != nil {
		return
	}
	yNum, err := result.Results[1].(json.Number).Int64()
	if err != nil {
		return
	}
	colorNum, err := result.Results[2].(json.Number).Int64()
	if err != nil {
		return
	}
	x, y, color = int(xNum), int(yNum), int(colorNum)
	return
}

func getProposedMove(contractAddr string) (x, y, color int) {
	result, err := bcy.CallContract(bcyeth.Contract{Private: "c025000000000000000000000000000000000000000000000000000000000000"}, contractAddr, "proposed")
	if err != nil {
		return
	}
	xNum, err := result.Results[0].(json.Number).Int64()
	if err != nil {
		return
	}
	yNum, err := result.Results[1].(json.Number).Int64()
	if err != nil {
		return
	}
	colorNum, err := result.Results[2].(json.Number).Int64()
	if err != nil {
		return
	}
	x, y, color = int(xNum), int(yNum), int(colorNum)
	return
}
