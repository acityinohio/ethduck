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

	//constructor, requires boardsize and opponent
	function EthDuck(uint8 boardSize, address player2) payable {
		black = msg.sender;
		white = player2;
		size = boardSize;
	}
	//fallback function JIC
	function () payable
	onlyPlayers()
	{
	}

	//confirms game for white player; requires them to bet at least as much as black player
	function confirmNewGame() payable {
		if (msg.value < this.balance / 2 || msg.sender != white) {
			throw;
		} else {
			(confirmed, blackTurn) = (true, true);
		}
	}

	//allows black player to refund/self-destruct contract if not confirmed by white
	function refundGame() {
		if (msg.sender == black && !confirmed) {
			selfdestruct(black);
		}
	}

	//gets number of moves
	function getNumMoves() constant returns (uint _moves) {
		_moves = moves.length;
	}

	//gets n-th move, returns x,y,enum
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
	//modifier to restrict function to proposers
	modifier onlyPropose() { if (approvalLock || (msg.sender == black && !blackTurn) || (msg.sender == white && blackTurn)) { throw; } _; }
	//modifier to restrict function to authorizers
	modifier onlyAuthorize() { if (!approvalLock || (msg.sender == black && blackTurn) || (msg.sender == white && !blackTurn)) { throw; } _; }  
	//modifier to restrict size
	modifier onlySize(uint8 _x, uint8 _y) { if (_x >= size || _y >= size) { throw; } _; }

	//proposes move
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

	//other player authorizes move
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

	//proposes winner 
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

	//authorizes winner, ends contract
	function authorizeWinner(bool _approve)
	onlyPlayers()
	onlyAuthorize()
	{
		if (!_approve){
			delete winner;
			approvalLock = false;
		}
		if (winner == State.Black) {
			if (!black.send(3 * this.balance / 4)) {
				throw;
			}
			selfdestruct(white);
		} else {
			if (!white.send(3 * this.balance / 4)) {
				throw;
			}
			selfdestruct(black);
		}
	}

	//proposes draw
	function proposeDraw()
	onlyPlayers()
	onlyPropose()
	{
		draw = true;
		approvalLock = true;
	}

	//authorizes draw
	function authorizeDraw(bool _approve)
	onlyPlayers()
	onlyAuthorize()
	{
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
