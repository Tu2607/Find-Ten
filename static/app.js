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
  reconnectTimer: null,
  countdownTimer: null,
  startPending: false,
  startGeneration: 0,
  errorTimer: null
};

// Screen elements
const welcomeScreen = document.getElementById("welcomeScreen");
const settingsScreen = document.getElementById("settingsScreen");
const gameScreen = document.getElementById("gameScreen");

// Game elements
const boardEl = document.getElementById("board");
const scoreEl = document.getElementById("scoreValue");
const timeEl = document.getElementById("timeValue");
const reshuffleButtonEl = document.getElementById("reshuffleButton");
const removeNumberButtonEl = document.getElementById("removeNumberButton");
const hintButtonEl = document.getElementById("hintButton");

// Game over overlay elements
const gameOverOverlay = document.getElementById("gameOverOverlay");
const gameOverReasonEl = document.getElementById("gameOverReason");
const gameOverScoreEl = document.getElementById("gameOverScore");

// Screen navigation
const playButtonEl = document.getElementById("playButton");
playButtonEl.addEventListener("click", () => {
  const size = getSelectedBoardSize();
  showScreen("game");
  startGame(size);
});

document.getElementById("settingsButton").addEventListener("click", () => {
  showScreen("settings");
});

document.getElementById("settingsBackButton").addEventListener("click", () => {
  showScreen("welcome");
});

const playAgainButtonEl = document.getElementById("playAgainButton");
playAgainButtonEl.addEventListener("click", () => {
  hideGameOverOverlay();
  const size = getSelectedBoardSize();
  startGame(size);
});

document.getElementById("mainMenuButton").addEventListener("click", () => {
  state.startGeneration++;
  hideGameOverOverlay();
  closeStream();
  stopCountdown();
  if (state.gameId) abandonGame(state.gameId);
  state.gameId = null;
  showScreen("welcome");
});

document.getElementById("restartButton").addEventListener("click", () => {
  const size = getSelectedBoardSize();
  startGame(size);
});

document.getElementById("homeButton").addEventListener("click", () => {
  state.startGeneration++;
  closeStream();
  stopCountdown();
  if (state.gameId) abandonGame(state.gameId);
  state.gameId = null;
  showScreen("welcome");
});

reshuffleButtonEl.addEventListener("click", submitReshuffle);
removeNumberButtonEl.addEventListener("click", enableRemoveMode);
hintButtonEl.addEventListener("click", submitHint);

// Scatter chalk formulas randomly across the welcome chalkboard
(function scatterFormulas() {
  const formulas = [
    "y = ax + b", "A = πr²", "a² + b² = c²", "C = 2πr",
    "E = mc²", "V = lwh", "f(x) = x²", "d = rt",
    "Σ = n(n+1)/2", "sin²θ + cos²θ = 1",
    "log(ab) = log a + log b", "tanθ = sinθ/cosθ",
    "x = (-b ± √(b²-4ac)) / 2a", "∫ x dx = x²/2 + C",
    "n! = n × (n-1)!", "KE = ½mv²", "F = ma", "PV = nRT",
    "P(A∪B) = P(A) + P(B)", "Δy / Δx"
  ];
  const container = document.getElementById("chalkFormulas");
  if (!container) return;
  const placed = [];

  for (const text of formulas) {
    const span = document.createElement("span");
    span.textContent = text;
    const rot = (Math.random() * 14 - 7).toFixed(1);
    let top, left, tries = 0;
    do {
      top = Math.random() * 88 + 3;
      left = Math.random() * 80 + 3;
      tries++;
    } while (tries < 40 && (isCenter(top, left) || tooClose(top, left, placed)));
    placed.push({ top, left });
    span.style.cssText = `top:${top.toFixed(1)}%;left:${left.toFixed(1)}%;transform:rotate(${rot}deg)`;
    container.appendChild(span);
  }

  function isCenter(t, l) {
    return t > 25 && t < 80 && l > 25 && l < 65;
  }

  function tooClose(t, l, list) {
    return list.some(p => Math.abs(p.top - t) < 7 && Math.abs(p.left - l) < 12);
  }
})();

function showScreen(name) {
  hideError();
  welcomeScreen.hidden = name !== "welcome";
  settingsScreen.hidden = name !== "settings";
  gameScreen.hidden = name !== "game";
}

function getSelectedBoardSize() {
  const checked = document.querySelector('input[name="boardSize"]:checked');
  return checked ? Number(checked.value) : 9;
}

function showGameOverOverlay() {
  gameOverReasonEl.textContent = gameOverReasonText(state.gameOverReason);
  gameOverScoreEl.textContent = state.score;
  gameOverOverlay.hidden = false;
}

function hideGameOverOverlay() {
  gameOverOverlay.hidden = true;
}

async function startGame(size) {
  if (state.startPending) return;
  state.startPending = true;
  playButtonEl.disabled = true;
  playAgainButtonEl.disabled = true;

  try {
    const gen = ++state.startGeneration;
    const previousGameId = state.gameId;

    closeStream();
    stopCountdown();
    hideGameOverOverlay();

    state.gameId = null;
    if (previousGameId) {
      await abandonGame(previousGameId);
      if (gen !== state.startGeneration) return;
    }

    let response;
    try {
      response = await fetch("/games", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ size })
      });
    } catch {
      if (gen !== state.startGeneration) return;
      showScreen("welcome");
      showError("Could not connect to the server.");
      return;
    }

    if (gen !== state.startGeneration) return;

    if (!response.ok) {
      showScreen("welcome");
      showError("Failed to start a new game.");
      return;
    }

    let game;
    try {
      game = await response.json();
    } catch {
      if (gen !== state.startGeneration) return;
      showScreen("welcome");
      showError("Invalid response from server.");
      return;
    }

    if (gen !== state.startGeneration) {
      if (game && game.gameId) abandonGame(game.gameId);
      return;
    }

    state.gameId = game.gameId;
    state.expiresAt = new Date(game.expiresAt);
    state.gameOver = false;
    state.gameOverReason = 0;
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
  } finally {
    state.startPending = false;
    playButtonEl.disabled = false;
    playAgainButtonEl.disabled = false;
  }
}

async function abandonGame(gameId) {
  try {
    await fetch(`/games/${gameId}`, {
      method: "DELETE"
    });
  } catch {
    // Best effort only.
  }
}

function openSnapshots(gameId) {
  state.eventSource = new EventSource(`/games/${gameId}/snapshots`);
  state.eventSource.addEventListener("snapshot", (event) => {
    if (state.gameId !== gameId) return;
    let snapshot;
    try { snapshot = JSON.parse(event.data); } catch { return; }
    applySnapshot(snapshot);
  });
  state.eventSource.onerror = () => {
    if (state.gameOver || state.gameId !== gameId) return;
    closeStream();
    state.reconnectTimer = setTimeout(() => {
      state.reconnectTimer = null;
      if (!state.gameOver && state.gameId === gameId) {
        openSnapshots(gameId);
      }
    }, 2000);
  };
}

function endGame(reason) {
  revertOptimisticMove();
  state.gameOver = true;
  state.gameOverReason = reason || state.gameOverReason || 0;
  closeStream();
  stopCountdown();
  updateSkillButtons();
  renderBoard();
  if (!state.gameOverPopupShown) {
    state.gameOverPopupShown = true;
    showGameOverOverlay();
  }
}

function showError(message) {
  const el = document.getElementById("errorToast");
  el.textContent = message;
  el.hidden = false;
  clearTimeout(state.errorTimer);
  state.errorTimer = setTimeout(() => { el.hidden = true; state.errorTimer = null; }, 4000);
}

function hideError() {
  const el = document.getElementById("errorToast");
  el.hidden = true;
  clearTimeout(state.errorTimer);
  state.errorTimer = null;
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

  endGame(snapshot.gameOverReason);
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

  let response;
  try {
    response = await fetch(`/games/${state.gameId}/reshuffle`, {
      method: "POST"
    });
  } catch {
    state.reshufflePending = false;
    updateSkillButtons();
    showError("Reshuffle failed — check your connection.");
    return;
  }

  if (response.ok) {
    return;
  }

  state.reshufflePending = false;
  if (response.status === 409 || response.status === 410) {
    state.reshuffleUsed = true;
    updateSkillButtons();
    if (response.status === 410) endGame();
    return;
  }

  if (response.status >= 500) showError("Server error — try again.");
  updateSkillButtons();
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

  let response;
  try {
    response = await fetch(`/games/${state.gameId}/hint`, {
      method: "POST"
    });
  } catch {
    state.hintPending = false;
    updateSkillButtons();
    showError("Hint failed — check your connection.");
    return;
  }

  if (response.ok) {
    let body;
    try {
      body = await response.json();
    } catch {
      state.hintPending = false;
      updateSkillButtons();
      showError("Hint failed — try again.");
      return;
    }
    if (!body.selection || !body.selection.start || !body.selection.end) {
      state.hintPending = false;
      updateSkillButtons();
      showError("Hint failed — try again.");
      return;
    }
    const hintSnapshotAlreadyReceived = state.hintUsed;
    state.hintHighlight = body.selection;
    state.hintSnapshotPending = !hintSnapshotAlreadyReceived;
    state.hintUsed = true;
    state.hintPending = false;
    updateSkillButtons();
    renderBoard();
    return;
  }

  state.hintPending = false;
  if (response.status === 409 || response.status === 410) {
    updateSkillButtons();
    if (response.status === 410) {
      state.hintHighlight = null;
      state.hintSnapshotPending = false;
      endGame();
    }
    return;
  }

  if (response.status >= 500) showError("Server error — try again.");
  updateSkillButtons();
}

function enableRemoveMode() {
  if (!state.gameId || state.gameOver || state.removeNumberUsed || state.removeNumberPending) {
    return;
  }
  if (state.removeMode) {
    state.removeMode = false;
    updateSkillButtons();
    return;
  }

  state.removeMode = true;
  state.selectedStart = null;
  state.hoverCell = null;
  updateSelectionPreview();
  updateSkillButtons();
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
    return;
  }

  state.removeNumberPending = true;
  state.selectedStart = null;
  state.hoverCell = null;
  updateSelectionPreview();
  updateSkillButtons();

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
    showError("Remove failed — check your connection.");
    return;
  }

  if (response.ok) {
    return;
  }

  state.removeNumberPending = false;
  if (response.status === 409 || response.status === 410) {
    state.removeNumberUsed = true;
    state.removeMode = false;
    updateSkillButtons();
    if (response.status === 410) endGame();
    return;
  }

  if (response.status >= 500) showError("Server error — try again.");
  state.removeMode = false;
  updateSkillButtons();
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
    showError("Move failed — check your connection.");
    return;
  }

  if (response.ok) {
    return;
  }

  revertOptimisticMove();

  if (response.status === 409 || response.status === 410) {
    endGame();
    return;
  }

  if (response.status >= 500) showError("Server error — try again.");
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
    endGame(2);
  }
}

function stopCountdown() {
  if (state.countdownTimer) {
    clearInterval(state.countdownTimer);
  }
  state.countdownTimer = null;
}

function closeStream() {
  if (state.reconnectTimer) {
    clearTimeout(state.reconnectTimer);
    state.reconnectTimer = null;
  }
  if (state.eventSource) {
    state.eventSource.close();
  }
  state.eventSource = null;
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

function gameOverReasonText(reason) {
  switch (reason) {
    case 1:
      return "No Moves Left!";
    case 2:
      return "Time's Up!";
    case 3:
      return "Board Cleared!";
    default:
      return "Game Over!";
  }
}
