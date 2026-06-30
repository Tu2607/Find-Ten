const state = {
  gameId: null,
  board: [],
  score: 0,
  gameOver: false,
  gameOverReason: 0,
  gameOverPopupShown: false,
  reshuffleUsed: false,
  reshufflePending: false,
  removeNumberUsed: false,
  removeNumberPending: false,
  removeMode: false,
  hintUsed: false,
  hintPending: false,
  hintHighlight: null,
  hintSnapshotPending: false,
  selectedStart: null,
  hoverCell: null,
  optimisticMove: null,
  cellRefs: [],
  lastBoardSize: 0,
  lastBoardWidth: 0,
  expiresAt: null,
  eventSource: null,
  countdownTimer: null
};

const boardEl = document.getElementById("board");
const scoreEl = document.getElementById("scoreValue");
const timeEl = document.getElementById("timeValue");
const gameEl = document.getElementById("gameValue");
const statusEl = document.getElementById("statusText");
const reshuffleButtonEl = document.getElementById("reshuffleButton");
const removeNumberButtonEl = document.getElementById("removeNumberButton");
const hintButtonEl = document.getElementById("hintButton");

document.getElementById("startForm").addEventListener("submit", async (event) => {
  event.preventDefault();
  await startGame(Number(document.getElementById("boardSize").value));
});

reshuffleButtonEl.addEventListener("click", submitReshuffle);
removeNumberButtonEl.addEventListener("click", enableRemoveMode);
hintButtonEl.addEventListener("click", submitHint);

async function startGame(size) {
  const previousGameId = state.gameId;

  closeStream();
  stopCountdown();
  setStatus("Starting game...");
  gameEl.textContent = "Starting";

  if (previousGameId) {
    await abandonGame(previousGameId);
  }

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
  state.reshuffleUsed = false;
  state.reshufflePending = false;
  state.removeNumberUsed = false;
  state.removeNumberPending = false;
  state.removeMode = false;
  state.hintUsed = false;
  state.hintPending = false;
  state.hintHighlight = null;
  state.hintSnapshotPending = false;
  state.optimisticMove = null;
  state.cellRefs = [];
  state.lastBoardSize = 0;
  state.lastBoardWidth = 0;

  applySnapshot(game.initialSnapshot);
  openSnapshots(game.gameId);
  startCountdown();
  setStatus("Select two corners that sum to 10.");
}

async function abandonGame(gameId) {
  try {
    await fetch(`/games/${gameId}`, {
      method: "DELETE"
    });
  } catch {
    // Best effort only. Starting a new game should still be attempted.
  }
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
  let optimisticConfirmed = false;
  if (state.optimisticMove) {
    optimisticConfirmed = snapshot.gameOver ||
      snapshot.validMoveCount === 0 ||
      state.optimisticMove.affectedCells.every(({ row, col }) => {
        const rowValues = snapshot.board[row];
        return rowValues && rowValues[col] === 0;
      });
  }

  state.board = snapshot.board;
  state.score = snapshot.score;
  state.gameOver = snapshot.gameOver;
  state.gameOverReason = snapshot.gameOverReason;
  state.reshuffleUsed = Boolean(snapshot.reshuffleUsed);
  state.removeNumberUsed = Boolean(snapshot.removeNumberUsed);
  state.hintUsed = Boolean(snapshot.hintUsed);
  state.reshufflePending = false;
  state.removeNumberPending = false;
  state.hintPending = false;
  if (state.hintHighlight && state.hintSnapshotPending && snapshot.hintUsed) {
    state.hintSnapshotPending = false;
  } else {
    state.hintHighlight = null;
    state.hintSnapshotPending = false;
  }
  state.removeMode = false;
  state.selectedStart = null;
  state.hoverCell = null;

  if (snapshot.gameOver || snapshot.validMoveCount === 0) {
    state.optimisticMove = null;
    state.gameOver = true;
    stopCountdown();
    timeEl.textContent = "0s";
  } else if (optimisticConfirmed) {
    state.optimisticMove = null;
  }

  updateScoreDisplay();
  gameEl.textContent = state.gameOver ? gameOverText(state.gameOverReason) : "Playing";
  updateSkillButtons();
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
  boardEl.classList.toggle("empty", state.board.length === 0);
  boardEl.style.setProperty("--size", state.board.length || 9);

  const boardWidth = currentBoardWidth();
  const needsFullBuild = state.cellRefs.length === 0 ||
    state.board.length !== state.lastBoardSize ||
    boardWidth !== state.lastBoardWidth ||
    boardWidth !== currentCellRefWidth();
  if (needsFullBuild) {
    boardEl.innerHTML = "";
    state.cellRefs = [];

    state.board.forEach((row, rowIndex) => {
      state.cellRefs[rowIndex] = [];
      row.forEach((_, colIndex) => {
        const cell = document.createElement("button");
        cell.type = "button";
        cell.dataset.row = rowIndex;
        cell.dataset.col = colIndex;
        cell.addEventListener("click", () => handleCellClick(rowIndex, colIndex));
        cell.addEventListener("mouseenter", () => handleCellHover(rowIndex, colIndex));
        cell.addEventListener("focus", () => handleCellHover(rowIndex, colIndex));
        state.cellRefs[rowIndex][colIndex] = cell;
        boardEl.appendChild(cell);
      });
    });
    state.lastBoardSize = state.board.length;
    state.lastBoardWidth = boardWidth;
  }

  state.board.forEach((row, rowIndex) => {
    row.forEach((value, colIndex) => {
      const cell = state.cellRefs[rowIndex] && state.cellRefs[rowIndex][colIndex];
      if (!cell) {
        return;
      }
      const text = value === 0 ? "" : String(value);
      if (cell.textContent !== text) {
        cell.textContent = text;
      }
      updateCellClasses(cell, rowIndex, colIndex, value);
      if (cell.disabled !== state.gameOver) {
        cell.disabled = state.gameOver;
      }
    });
  });

  updateSelectionPreview();
}

function updateCellClasses(cell, row, col, value) {
  const classes = ["cell"];
  if (value === 0) {
    classes.push("cleared");
  }
  if (value !== 0 && state.hintHighlight && positionInsideSelection({ row, col }, state.hintHighlight)) {
    classes.push("hint");
  }
  if (isOptimisticallyClearing(row, col)) {
    classes.push("clearing");
  }

  const nextClassName = classes.join(" ");
  if (cell.className !== nextClassName) {
    cell.className = nextClassName;
  }
}

function currentBoardWidth() {
  return state.board.length > 0 && state.board[0] ? state.board[0].length : 0;
}

function currentCellRefWidth() {
  return state.cellRefs.length > 0 && state.cellRefs[0] ? state.cellRefs[0].length : 0;
}

async function submitReshuffle() {
  if (!state.gameId || state.gameOver || state.reshuffleUsed || state.reshufflePending) {
    return;
  }

  state.reshufflePending = true;
  state.removeMode = false;
  state.selectedStart = null;
  state.hoverCell = null;
  updateSelectionPreview();
  updateSkillButtons();
  setStatus("Reshuffling board...");

  let response;
  try {
    response = await fetch(`/games/${state.gameId}/reshuffle`, {
      method: "POST"
    });
  } catch {
    state.reshufflePending = false;
    updateSkillButtons();
    setStatus("Could not reshuffle.");
    return;
  }

  if (response.ok) {
    setStatus("Reshuffle accepted.");
    return;
  }

  state.reshufflePending = false;
  if (response.status === 409 || response.status === 410) {
    state.reshuffleUsed = true;
    if (response.status === 410) {
      revertOptimisticMove();
      state.gameOver = true;
      gameEl.textContent = "Ended";
      renderBoard();
    }
    updateSkillButtons();
    setStatus(response.status === 409 ? "Reshuffle unavailable." : "Game ended.");
    return;
  }

  updateSkillButtons();
  setStatus("Reshuffle failed.");
}

async function submitHint() {
  if (!state.gameId || state.gameOver || state.hintUsed || state.hintPending) {
    return;
  }

  state.hintPending = true;
  state.removeMode = false;
  state.selectedStart = null;
  state.hoverCell = null;
  updateSelectionPreview();
  updateSkillButtons();
  setStatus("Finding hint...");

  let response;
  try {
    response = await fetch(`/games/${state.gameId}/hint`, {
      method: "POST"
    });
  } catch {
    state.hintPending = false;
    updateSkillButtons();
    setStatus("Could not get hint.");
    return;
  }

  if (response.ok) {
    let body;
    try {
      body = await response.json();
    } catch {
      state.hintPending = false;
      updateSkillButtons();
      setStatus("Hint failed.");
      return;
    }
    if (!body.selection || !body.selection.start || !body.selection.end) {
      state.hintPending = false;
      updateSkillButtons();
      setStatus("Hint failed.");
      return;
    }
    const hintSnapshotAlreadyReceived = state.hintUsed;
    state.hintHighlight = body.selection;
    state.hintSnapshotPending = !hintSnapshotAlreadyReceived;
    state.hintUsed = true;
    state.hintPending = false;
    updateSkillButtons();
    renderBoard();
    setStatus("Hint shown.");
    return;
  }

  state.hintPending = false;
  if (response.status === 409 || response.status === 410) {
    if (response.status === 410) {
      revertOptimisticMove();
      state.gameOver = true;
      state.hintHighlight = null;
      state.hintSnapshotPending = false;
      gameEl.textContent = "Ended";
      stopCountdown();
      renderBoard();
    }
    updateSkillButtons();
    setStatus(response.status === 409 ? "Hint unavailable." : "Game ended.");
    return;
  }

  updateSkillButtons();
  setStatus("Hint failed.");
}

function enableRemoveMode() {
  if (!state.gameId || state.gameOver || state.removeNumberUsed || state.removeNumberPending) {
    return;
  }
  if (state.removeMode) {
    state.removeMode = false;
    updateSkillButtons();
    setStatus("Remove canceled.");
    return;
  }

  state.removeMode = true;
  state.selectedStart = null;
  state.hoverCell = null;
  updateSelectionPreview();
  updateSkillButtons();
  setStatus("Choose a number to remove.");
}

async function handleCellClick(row, col) {
  if (!state.gameId || state.gameOver) {
    return;
  }

  if (state.removeMode) {
    await submitRemoveNumber({ row, col });
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
  applyOptimisticMove(selection);
  await submitMove(selection);
}

async function submitRemoveNumber(position) {
  if (state.removeNumberPending || state.removeNumberUsed) {
    return;
  }
  if (state.board[position.row][position.col] === 0) {
    setStatus("Choose a number that has not been cleared.");
    return;
  }

  state.removeNumberPending = true;
  state.selectedStart = null;
  state.hoverCell = null;
  updateSelectionPreview();
  updateSkillButtons();
  setStatus("Removing number...");

  let response;
  try {
    response = await fetch(`/games/${state.gameId}/remove-number`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ position })
    });
  } catch {
    state.removeNumberPending = false;
    state.removeMode = false;
    updateSkillButtons();
    setStatus("Could not remove number.");
    return;
  }

  if (response.ok) {
    setStatus("Remove accepted.");
    return;
  }

  state.removeNumberPending = false;
  if (response.status === 409 || response.status === 410) {
    state.removeNumberUsed = true;
    state.removeMode = false;
    if (response.status === 410) {
      revertOptimisticMove();
      state.gameOver = true;
      gameEl.textContent = "Ended";
      renderBoard();
    }
    updateSkillButtons();
    setStatus(response.status === 409 ? "Remove unavailable." : "Game ended.");
    return;
  }

  state.removeMode = false;
  updateSkillButtons();
  setStatus(response.status === 400 ? "Cannot remove that cell." : "Remove failed.");
}

function handleCellHover(row, col) {
  if (!state.selectedStart || state.removeMode || state.gameOver) {
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
    revertOptimisticMove();
    setStatus("Could not submit move.");
    return;
  }

  if (response.ok) {
    setStatus("Move accepted.");
    return;
  }

  revertOptimisticMove();

  if (response.status === 400) {
    setStatus("That rectangle does not make 10.");
    return;
  }

  if (response.status === 409 || response.status === 410) {
    state.gameOver = true;
    gameEl.textContent = "Ended";
    setStatus("Game ended.");
    updateSkillButtons();
    renderBoard();
    return;
  }

  setStatus("Move failed.");
}

function applyOptimisticMove(selection) {
  revertOptimisticMove();

  const minRow = Math.min(selection.start.row, selection.end.row);
  const maxRow = Math.max(selection.start.row, selection.end.row);
  const minCol = Math.min(selection.start.col, selection.end.col);
  const maxCol = Math.max(selection.start.col, selection.end.col);
  const affectedCells = [];

  for (let row = minRow; row <= maxRow; row += 1) {
    for (let col = minCol; col <= maxCol; col += 1) {
      if (state.board[row][col] !== 0) {
        affectedCells.push({ row, col });
      }
    }
  }

  if (affectedCells.length === 0) {
    return;
  }

  state.optimisticMove = {
    affectedCells,
    affectedCellKeys: new Set(affectedCells.map(cellKey)),
    scoreAdded: affectedCells.length * 100
  };

  affectedCells.forEach(({ row, col }) => {
    const cell = state.cellRefs[row] && state.cellRefs[row][col];
    if (cell) {
      cell.classList.add("clearing");
    }
  });
  updateScoreDisplay();
}

function revertOptimisticMove() {
  if (!state.optimisticMove) {
    return;
  }

  state.optimisticMove.affectedCells.forEach(({ row, col }) => {
    const cell = state.cellRefs[row] && state.cellRefs[row][col];
    if (cell) {
      cell.classList.remove("clearing");
    }
  });
  state.optimisticMove = null;
  updateScoreDisplay();
}

function isOptimisticallyClearing(row, col) {
  return Boolean(state.optimisticMove && state.optimisticMove.affectedCellKeys.has(cellKey({ row, col })));
}

function updateScoreDisplay() {
  const optimisticScore = state.optimisticMove ? state.optimisticMove.scoreAdded : 0;
  scoreEl.textContent = state.score + optimisticScore;
}

function updateSelectionPreview() {
  forEachCellRef((cell) => cell.classList.remove("selected", "corner"));

  if (!state.selectedStart) {
    return;
  }

  const start = state.selectedStart;
  const end = state.hoverCell || start;
  const minRow = Math.min(start.row, end.row);
  const maxRow = Math.max(start.row, end.row);
  const minCol = Math.min(start.col, end.col);
  const maxCol = Math.max(start.col, end.col);

  forEachCellRef((cell) => {
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

function forEachCellRef(callback) {
  state.cellRefs.forEach((row) => {
    if (!row) {
      return;
    }
    row.forEach((cell) => {
      if (cell) {
        callback(cell);
      }
    });
  });
}

function cellKey(cell) {
  return `${cell.row},${cell.col}`;
}

function positionInsideSelection(position, selection) {
  const minRow = Math.min(selection.start.row, selection.end.row);
  const maxRow = Math.max(selection.start.row, selection.end.row);
  const minCol = Math.min(selection.start.col, selection.end.col);
  const maxCol = Math.max(selection.start.col, selection.end.col);

  return position.row >= minRow &&
    position.row <= maxRow &&
    position.col >= minCol &&
    position.col <= maxCol;
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
    revertOptimisticMove();
    state.gameOver = true;
    state.gameOverReason = 2;
    state.gameOverPopupShown = true;
    gameEl.textContent = "Expired";
    setStatus("Time expired.");
    updateSkillButtons();
    renderBoard();
    alert("Game over: Expired");
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

function updateSkillButtons() {
  updateReshuffleButton();
  updateRemoveNumberButton();
  updateHintButton();
}

function updateReshuffleButton() {
  const disabled = !state.gameId || state.gameOver || state.reshuffleUsed || state.reshufflePending;
  reshuffleButtonEl.disabled = disabled;

  if (state.reshufflePending) {
    reshuffleButtonEl.textContent = "Reshuffling";
    return;
  }
  if (state.reshuffleUsed) {
    reshuffleButtonEl.textContent = "Used";
    return;
  }

  reshuffleButtonEl.textContent = "Reshuffle";
}

function updateRemoveNumberButton() {
  const disabled = !state.gameId || state.gameOver || state.removeNumberUsed || state.removeNumberPending;
  removeNumberButtonEl.disabled = disabled;

  if (state.removeNumberPending) {
    removeNumberButtonEl.textContent = "Removing";
    return;
  }
  if (state.removeNumberUsed) {
    removeNumberButtonEl.textContent = "Used";
    return;
  }
  if (state.removeMode) {
    removeNumberButtonEl.textContent = "Cancel";
    return;
  }

  removeNumberButtonEl.textContent = "Remove";
}

function updateHintButton() {
  const disabled = !state.gameId || state.gameOver || state.hintUsed || state.hintPending;
  hintButtonEl.disabled = disabled;

  if (state.hintPending) {
    hintButtonEl.textContent = "Hinting";
    return;
  }
  if (state.hintUsed) {
    hintButtonEl.textContent = "Used";
    return;
  }

  hintButtonEl.textContent = "Hint";
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
