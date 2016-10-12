pragma solidity ^0.4.0;

/// @title EthDuck
/// @author acityinohio
contract EthDuck {
	uint8 public size;
	address public black;
	address public white;
	bool public confirmed;
	bool public blackTurn;
	bool public approvalLock;
	bool public draw;
	enum State { Empty, Black, White }
	struct Move {
		uint8 x;
		uint8 y;
		State color;
	}
	Move[] moves;
	Move public proposed;
	State public winner;

	//constructor for initializing contract, requires boardsize and opponent/"white" stone address
	//note the "payable" identifier; new to Solidity 0.4.0 for any contract methods that accept wei
	//the address that constructs the contract is the "black" stone player
	//and their initial wager is the value sent with the constructor
	//winner gets 3/4th of the pot, loser gets 1/4th, or they draw and split it
	function EthDuck(uint8 boardSize, address player2) payable {
		black = msg.sender;
		white = player2;
		size = boardSize;
	}
	//make fallback function payable just in case players want to add more to the pot
	//uses onlyPlayers modifier to ensure no one else can send value to this contract
	function () payable
	onlyPlayers()
	{
	}

	//confirms game for white player; requires them to bet at least as much as black player
	//"confirm" bool is used as a check for "refundGame" below
	//also sets blackTurn to true, to let black move first
	function confirmNewGame() payable {
		if (msg.value < this.balance / 2 || msg.sender != white) {
			throw;
		} else {
			(confirmed, blackTurn) = (true, true);
		}
	}

	//allows black player to refund/self-destruct contract if white doesn't confirmNewGame()
	function refundGame() {
		if (msg.sender == black && !confirmed) {
			selfdestruct(black);
		}
	}

	//gets number of moves
	function getNumMoves() constant returns (uint _moves) {
		_moves = moves.length;
	}

	//gets n-th move, returns x,y,State
	function getMove(uint _n) constant returns (uint8 _x, uint8 _y, State _color) {
		if (_n >= moves.length) {
			throw;
		}
		_x = moves[_n].x;
		_y = moves[_n].y;
		_color = moves[_n].color;
	}

	//modifier to restrict moves to players
	modifier onlyPlayers() { if (msg.sender != black && msg.sender != white) { throw; } _; }
	//modifier to restrict function to players when its their turn to propose
	modifier onlyPropose() { if (approvalLock || (msg.sender == black && !blackTurn) || (msg.sender == white && blackTurn)) { throw; } _; }
	//modifier to restrict function to authorizers
	modifier onlyAuthorize() { if (!approvalLock || (msg.sender == black && blackTurn) || (msg.sender == white && !blackTurn)) { throw; } _; }  
	//modifier to restrict size
	modifier onlySize(uint8 _x, uint8 _y) { if (_x >= size || _y >= size) { throw; } _; }

	//proposes move for a player, and with the modifiers above, only on their turn and within the board
	function proposeMove(uint8 _x, uint8 _y)
	onlyPlayers()
	onlyPropose()
	onlySize(_x, _y)
	{
		approvalLock = true;
		State _color;
		if (msg.sender == black) {
			_color = State.Black;
		} else {
			_color = State.White;
		}
		proposed = Move(_x, _y, _color);
	}

	//a player can authorize move, when it's their turn
	//once they authorize, it's added to the "moves" array
	function authorizeMove(bool _approve) 
	onlyPlayers()
	onlyAuthorize()
	{
		if (_approve) {
			moves.push(proposed);
			blackTurn = !blackTurn;
		}
		delete proposed;
		approvalLock = false;
	}

	//after enough playtime, when it's their move, a player can propose that they won
	function proposeWinner()
	onlyPlayers()
	onlyPropose()
	{
		if (msg.sender == black) {
			winner = State.Black;
		} else {
			winner = State.White;
		}
		approvalLock = true;
	}

	//authorizes winner after proposed, self-destructs contract
	function authorizeWinner(bool _approve)
	onlyPlayers()
	onlyAuthorize()
	{
		//make sure winner has actually been proposed!
		if (winner == State.Empty) {
			return;
		}
		//if not approved, reset approval lock and delete winner
		if (!_approve){
			delete winner;
			approvalLock = false;
			return;
		}
		if (winner == State.Black) {
			if (!black.send(3 * this.balance / 4)) {
				throw;
			}
			selfdestruct(white);
		} else if (winner == State.White) {
			if (!white.send(3 * this.balance / 4)) {
				throw;
			}
			selfdestruct(black);
		}
	}

	//after enough playtime, when it's their move, a player can propose a draw
	function proposeDraw()
	onlyPlayers()
	onlyPropose()
	{
		draw = true;
		approvalLock = true;
	}

	//authorizes draw after proposed, self-destructs contract
	function authorizeDraw(bool _approve)
	onlyPlayers()
	onlyAuthorize()
	{
		if (!draw) {
			return;
		}
		if (_approve) {
			if (!black.send(this.balance / 2)) {
				throw;
			}
			selfdestruct(white);
		} else {
			draw = false;
			approvalLock = false;
		}
	}
}
