const state = {
  gameId: null,
  board: [],
  score: 0,
  gameOver: false,
  gameOverReason: 0,
  gameOverPopupShown: false,
  selectedStart: null,
  hoverCell: null,
  expiresAt: null,
  eventSource: null,
  countdownTimer: null
};

const boardEl = document.getElementById("board");
const scoreEl = document.getElementById("scoreValue");
const movesEl = document.getElementById("movesValue");
const timeEl = document.getElementById("timeValue");
const gameEl = document.getElementById("gameValue");
const statusEl = document.getElementById("statusText");

document.getElementById("startForm").addEventListener("submit", async (event) => {
  event.preventDefault();
  await startGame(Number(document.getElementById("boardSize").value));
});

async function startGame(size) {
  closeStream();
  stopCountdown();
  setStatus("Starting game...");
  gameEl.textContent = "Starting";

  let response;
  try {
    response = await fetch("/games", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ size })
    });
  } catch {
    setStatus("Could not reach the server.");
    gameEl.textContent = "Offline";
    return;
  }

  if (!response.ok) {
    setStatus("Could not start game.");
    gameEl.textContent = "Error";
    return;
  }

  const game = await response.json();
  state.gameId = game.gameId;
  state.expiresAt = new Date(game.expiresAt);
  state.selectedStart = null;
  state.hoverCell = null;
  state.gameOverPopupShown = false;

  applySnapshot(game.initialSnapshot);
  openSnapshots(game.gameId);
  startCountdown();
  setStatus("Select two corners that sum to 10.");
}

function openSnapshots(gameId) {
  state.eventSource = new EventSource(`/games/${gameId}/snapshots`);
  state.eventSource.addEventListener("snapshot", (event) => {
    applySnapshot(JSON.parse(event.data));
    setStatus("Board updated.");
  });
  state.eventSource.onerror = () => {
    if (!state.gameOver) {
      setStatus("Snapshot stream disconnected.");
    }
  };
}

function applySnapshot(snapshot) {
  state.board = snapshot.board;
  state.score = snapshot.score;
  state.gameOver = snapshot.gameOver;
  state.gameOverReason = snapshot.gameOverReason;
  state.selectedStart = null;
  state.hoverCell = null;

  if (snapshot.gameOver || snapshot.validMoveCount === 0) {
    state.gameOver = true;
    stopCountdown();
    timeEl.textContent = "0s";
  }

  scoreEl.textContent = state.score;
  movesEl.textContent = snapshot.validMoveCount;
  gameEl.textContent = state.gameOver ? gameOverText(state.gameOverReason) : "Playing";
  maybeShowGameOver(snapshot);
  renderBoard();
}

function maybeShowGameOver(snapshot) {
  const noMovesLeft = snapshot.validMoveCount === 0;
  const isOver = snapshot.gameOver || noMovesLeft;

  if (!isOver || state.gameOverPopupShown) {
    return;
  }

  const reason = snapshot.gameOverReason || 1;
  state.gameOverPopupShown = true;
  state.gameOver = true;
  state.gameOverReason = reason;
  gameEl.textContent = gameOverText(reason);
  alert(`Game over: ${gameOverText(reason)}`);
}

function renderBoard() {
  boardEl.innerHTML = "";
  boardEl.classList.toggle("empty", state.board.length === 0);
  boardEl.style.setProperty("--size", state.board.length || 9);

  state.board.forEach((row, rowIndex) => {
    row.forEach((value, colIndex) => {
      const cell = document.createElement("button");
      cell.type = "button";
      cell.className = "cell";
      cell.textContent = value === 0 ? "" : String(value);
      cell.dataset.row = rowIndex;
      cell.dataset.col = colIndex;
      cell.disabled = state.gameOver;

      if (value === 0) {
        cell.classList.add("cleared");
      }
      cell.addEventListener("click", () => handleCellClick(rowIndex, colIndex));
      cell.addEventListener("mouseenter", () => handleCellHover(rowIndex, colIndex));
      cell.addEventListener("focus", () => handleCellHover(rowIndex, colIndex));
      boardEl.appendChild(cell);
    });
  });

  updateSelectionPreview();
}

async function handleCellClick(row, col) {
  if (!state.gameId || state.gameOver) {
    return;
  }

  if (!state.selectedStart) {
    state.selectedStart = { row, col };
    state.hoverCell = { row, col };
    updateSelectionPreview();
    setStatus("Choose the opposite corner.");
    return;
  }

  const selection = { start: state.selectedStart, end: { row, col } };
  state.selectedStart = null;
  state.hoverCell = null;
  updateSelectionPreview();
  await submitMove(selection);
}

function handleCellHover(row, col) {
  if (!state.selectedStart || state.gameOver) {
    return;
  }

  state.hoverCell = { row, col };
  updateSelectionPreview();
}

async function submitMove(selection) {
  let response;
  try {
    response = await fetch(`/games/${state.gameId}/moves`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ selection })
    });
  } catch {
    setStatus("Could not submit move.");
    return;
  }

  if (response.ok) {
    setStatus("Move accepted.");
    return;
  }

  if (response.status === 400) {
    setStatus("That rectangle does not make 10.");
    return;
  }

  if (response.status === 409 || response.status === 410) {
    state.gameOver = true;
    gameEl.textContent = "Ended";
    setStatus("Game ended.");
    renderBoard();
    return;
  }

  setStatus("Move failed.");
}

function updateSelectionPreview() {
  const cells = boardEl.querySelectorAll(".cell");
  cells.forEach((cell) => cell.classList.remove("selected", "corner"));

  if (!state.selectedStart) {
    return;
  }

  const start = state.selectedStart;
  const end = state.hoverCell || start;
  const minRow = Math.min(start.row, end.row);
  const maxRow = Math.max(start.row, end.row);
  const minCol = Math.min(start.col, end.col);
  const maxCol = Math.max(start.col, end.col);

  cells.forEach((cell) => {
    const row = Number(cell.dataset.row);
    const col = Number(cell.dataset.col);
    if (row >= minRow && row <= maxRow && col >= minCol && col <= maxCol) {
      cell.classList.add("selected");
    }
    if (row === start.row && col === start.col) {
      cell.classList.add("corner");
    }
    if (row === end.row && col === end.col) {
      cell.classList.add("corner");
    }
  });
}

function startCountdown() {
  renderCountdown();
  state.countdownTimer = setInterval(renderCountdown, 250);
}

function renderCountdown() {
  if (!state.expiresAt) {
    timeEl.textContent = "--";
    return;
  }

  const remainingMs = state.expiresAt.getTime() - Date.now();
  const remaining = Math.max(0, Math.ceil(remainingMs / 1000));
  timeEl.textContent = `${remaining}s`;

  if (remaining === 0 && !state.gameOver) {
    state.gameOver = true;
    gameEl.textContent = "Expired";
    setStatus("Time expired.");
    renderBoard();
  }
}

function stopCountdown() {
  if (state.countdownTimer) {
    clearInterval(state.countdownTimer);
  }
  state.countdownTimer = null;
}

function closeStream() {
  if (state.eventSource) {
    state.eventSource.close();
  }
  state.eventSource = null;
}

function setStatus(message) {
  statusEl.textContent = message;
}

function gameOverText(reason) {
  switch (reason) {
    case 1:
      return "No moves";
    case 2:
      return "Expired";
    case 3:
      return "Cleared";
    default:
      return "Ended";
  }
}
