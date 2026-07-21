const state = {
  gameId: null,
  board: [],
  score: 0,
  gameOver: false,
  gameOverReason: 0,
  gameOverPopupShown: false,
  scoreSubmitted: false,
  scoreSubmitPending: false,
  scoreSubmitGameId: null,
  scoreSubmitController: null,
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
  leaderboardBoardSize: 9,
  leaderboardDuration: 120,
  leaderboardScores: [],
  leaderboardLoading: false,
  leaderboardError: false,
  leaderboardRequestId: 0,
  player: null,
  accountView: "guest",
  registrationHandle: "",
  authRequestPending: false,
  authStateVersion: 0,
  errorTimer: null
};

// Screen elements
const welcomeScreen = document.getElementById("welcomeScreen");
const settingsScreen = document.getElementById("settingsScreen");
const leaderboardScreen = document.getElementById("leaderboardScreen");
const gameScreen = document.getElementById("gameScreen");

// Leaderboard elements
const leaderboardBodyEl = document.getElementById("leaderboardBody");
const leaderboardEmptyEl = document.getElementById("leaderboardEmpty");
const leaderboardFilterSummaryEl = document.getElementById("leaderboardFilterSummary");

// Game elements
const boardEl = document.getElementById("board");
const scoreEl = document.getElementById("scoreValue");
const timeEl = document.getElementById("timeValue");
const reshuffleButtonEl = document.getElementById("reshuffleButton");
const removeNumberButtonEl = document.getElementById("removeNumberButton");
const hintButtonEl = document.getElementById("hintButton");
const colorSwatches = document.querySelectorAll(".color-swatches .swatch");
const validBoardColors = new Set(["green", "blue", "red", "purple"]);
const scoreRequestTimeoutMs = 10000;
const httpStatusUnauthorized = 401;

// Game over overlay elements
const gameOverOverlay = document.getElementById("gameOverOverlay");
const gameOverReasonEl = document.getElementById("gameOverReason");
const gameOverScoreEl = document.getElementById("gameOverScore");
const submitScoreOpenButtonEl = document.getElementById("submitScoreOpenButton");
const scoreSubmittedStatusEl = document.getElementById("scoreSubmittedStatus");
const scoreSubmissionOverlay = document.getElementById("scoreSubmissionOverlay");
const scoreSubmissionFormEl = document.getElementById("scoreSubmissionForm");
const playerNameLabelEl = document.getElementById("playerNameLabel");
const playerNameInputEl = document.getElementById("playerNameInput");
const submitScoreButtonEl = document.getElementById("submitScoreButton");
const cancelScoreSubmitButtonEl = document.getElementById("cancelScoreSubmitButton");
const scoreSubmitErrorEl = document.getElementById("scoreSubmitError");

// Account elements
const guestAccountActionsEl = document.getElementById("guestAccountActions");
const signedInAccountActionsEl = document.getElementById("signedInAccountActions");
const accountDisplayNameEl = document.getElementById("accountDisplayName");
const showLoginButtonEl = document.getElementById("showLoginButton");
const showRegisterButtonEl = document.getElementById("showRegisterButton");
const logoutButtonEl = document.getElementById("logoutButton");
const loginFormEl = document.getElementById("loginForm");
const loginHandleInputEl = document.getElementById("loginHandleInput");
const loginPasswordInputEl = document.getElementById("loginPasswordInput");
const loginErrorEl = document.getElementById("loginError");
const loginSubmitButtonEl = document.getElementById("loginSubmitButton");
const registrationFormEl = document.getElementById("registrationForm");
const registrationDisplayNameInputEl = document.getElementById("registrationDisplayNameInput");
const registrationPasswordInputEl = document.getElementById("registrationPasswordInput");
const registrationErrorEl = document.getElementById("registrationError");
const registrationSubmitButtonEl = document.getElementById("registrationSubmitButton");
const registrationSuccessEl = document.getElementById("registrationSuccess");
const registrationHandleEl = document.getElementById("registrationHandle");
const accountOverlayEl = document.getElementById("accountOverlay");

// Screen navigation
const playButtonEl = document.getElementById("playButton");
playButtonEl.addEventListener("click", () => {
  showScreen("game");
  startGame();
});

document.getElementById("settingsButton").addEventListener("click", () => {
  showScreen("settings");
});

document.getElementById("settingsBackButton").addEventListener("click", () => {
  showScreen("welcome");
});

document.getElementById("leaderboardButton").addEventListener("click", () => {
  showScreen("leaderboard");
  loadLeaderboardScores();
});

document.getElementById("leaderboardBackButton").addEventListener("click", () => {
  showScreen("welcome");
});

showLoginButtonEl.addEventListener("click", () => showAccountView("login"));
showRegisterButtonEl.addEventListener("click", () => showAccountView("register"));
document.getElementById("loginCancelButton").addEventListener("click", () => showAccountView("guest"));
document.getElementById("registrationCancelButton").addEventListener("click", () => showAccountView("guest"));
document.getElementById("registrationLoginButton").addEventListener("click", () => {
  showAccountView("login");
  loginHandleInputEl.value = state.registrationHandle;
  loginPasswordInputEl.focus();
});
loginFormEl.addEventListener("submit", submitLogin);
registrationFormEl.addEventListener("submit", submitRegistration);
logoutButtonEl.addEventListener("click", logout);

colorSwatches.forEach((swatch) => {
  swatch.addEventListener("click", () => {
    colorSwatches.forEach((item) => {
      const isActive = item === swatch;
      item.classList.toggle("swatch--active", isActive);
      item.setAttribute("aria-pressed", isActive ? "true" : "false");
    });
  });
});

leaderboardScreen.querySelectorAll(".chalk-pill-group").forEach((group) => {
  group.addEventListener("click", (event) => {
    const pill = event.target.closest(".chalk-pill");
    if (!pill) return;

    group.querySelectorAll(".chalk-pill").forEach((item) => {
      item.classList.remove("chalk-pill--active");
    });
    pill.classList.add("chalk-pill--active");
    updateLeaderboardFilter(group.dataset.filter, pill.dataset.value);
  });
});

const playAgainButtonEl = document.getElementById("playAgainButton");
playAgainButtonEl.addEventListener("click", () => {
  hideGameOverOverlay();
  hideScoreSubmissionOverlay();
  startGame();
});

document.getElementById("mainMenuButton").addEventListener("click", () => {
  state.startGeneration++;
  hideGameOverOverlay();
  hideScoreSubmissionOverlay();
  closeStream();
  stopCountdown();
  if (state.gameId) abandonGame(state.gameId);
  state.gameId = null;
  showScreen("welcome");
});

document.getElementById("restartButton").addEventListener("click", () => {
  hideScoreSubmissionOverlay();
  startGame();
});

document.getElementById("homeButton").addEventListener("click", () => {
  state.startGeneration++;
  hideScoreSubmissionOverlay();
  closeStream();
  stopCountdown();
  if (state.gameId) abandonGame(state.gameId);
  state.gameId = null;
  showScreen("welcome");
});

reshuffleButtonEl.addEventListener("click", submitReshuffle);
removeNumberButtonEl.addEventListener("click", enableRemoveMode);
hintButtonEl.addEventListener("click", submitHint);
submitScoreOpenButtonEl.addEventListener("click", showScoreSubmissionOverlay);
cancelScoreSubmitButtonEl.addEventListener("click", () => {
  hideScoreSubmissionOverlay();
  if (state.gameOver) showGameOverOverlay();
});
playerNameInputEl.addEventListener("input", updateScoreSubmitButton);
scoreSubmissionFormEl.addEventListener("submit", submitScore);

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

renderAccountState();
restoreAuthenticatedPlayer();

function showScreen(name) {
  hideError();
  if (name !== "welcome" && state.accountView !== "guest") {
    state.accountView = "guest";
    renderAccountState();
  }
  welcomeScreen.hidden = name !== "welcome";
  settingsScreen.hidden = name !== "settings";
  leaderboardScreen.hidden = name !== "leaderboard";
  gameScreen.hidden = name !== "game";
}

function showAccountView(view) {
  if (state.player) {
    state.accountView = "guest";
  } else {
    state.accountView = view;
  }
  hideLoginError();
  hideRegistrationError();
  renderAccountState();

  if (state.accountView === "login") {
    loginHandleInputEl.focus();
  } else if (state.accountView === "register") {
    registrationDisplayNameInputEl.focus();
  }
}

function renderAccountState() {
  const authenticated = state.player !== null;
  const accountOverlayVisible = !authenticated && state.accountView !== "guest";
  guestAccountActionsEl.hidden = authenticated || state.accountView !== "guest";
  signedInAccountActionsEl.hidden = !authenticated;
  accountOverlayEl.hidden = !accountOverlayVisible;
  loginFormEl.hidden = authenticated || state.accountView !== "login";
  registrationFormEl.hidden = authenticated || state.accountView !== "register";
  registrationSuccessEl.hidden = authenticated || state.accountView !== "registration-success";
  accountDisplayNameEl.textContent = authenticated ? state.player.displayName : "";
  registrationHandleEl.textContent = state.registrationHandle ? `Account handle: ${state.registrationHandle}` : "";
  updateAuthControls();
}

function updateAuthControls() {
  const pending = state.authRequestPending;
  loginHandleInputEl.disabled = pending;
  loginPasswordInputEl.disabled = pending;
  loginSubmitButtonEl.disabled = pending;
  registrationDisplayNameInputEl.disabled = pending;
  registrationPasswordInputEl.disabled = pending;
  registrationSubmitButtonEl.disabled = pending;
  logoutButtonEl.disabled = pending;
  loginSubmitButtonEl.textContent = pending && state.accountView === "login" ? "Logging In" : "Log In";
  registrationSubmitButtonEl.textContent = pending && state.accountView === "register" ? "Creating" : "Create";
}

async function restoreAuthenticatedPlayer() {
  const authStateVersion = state.authStateVersion;
  let response;
  try {
    response = await fetchWithTimeout("/auth/me", { credentials: "same-origin" });
  } catch {
    return;
  }

  if (response.status === httpStatusUnauthorized) {
    return;
  }
  if (!response.ok) {
    return;
  }

  const result = await readJSON(response);
  if (!result || !result.authenticated || !isPlayer(result.player)) {
    return;
  }
  if (authStateVersion !== state.authStateVersion || state.player !== null) {
    return;
  }

  setAuthenticatedPlayer(result.player);
}

async function submitLogin(event) {
  event.preventDefault();
  if (state.authRequestPending) return;

  const accountHandle = loginHandleInputEl.value.trim();
  const password = loginPasswordInputEl.value;
  if (!accountHandle || !password) {
    showLoginError("Enter your account handle and password.");
    return;
  }

  state.authRequestPending = true;
  hideLoginError();
  updateAuthControls();

  let response;
  try {
    response = await fetchWithTimeout("/auth/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      credentials: "same-origin",
      body: JSON.stringify({ accountHandle, password })
    });
  } catch (err) {
    state.authRequestPending = false;
    updateAuthControls();
    showLoginError(err && err.name === "AbortError" ? "Request timed out. Try again." : "Could not log in. Check your connection.");
    return;
  }

  const result = response.ok ? await readJSON(response) : null;
  state.authRequestPending = false;
  updateAuthControls();
  if (!response.ok || !result || !result.authenticated || !isPlayer(result.player)) {
    showLoginError(loginErrorMessage(response.status));
    return;
  }

  loginFormEl.reset();
  setAuthenticatedPlayer(result.player);
}

async function submitRegistration(event) {
  event.preventDefault();
  if (state.authRequestPending) return;

  const displayName = registrationDisplayNameInputEl.value.trim();
  const password = registrationPasswordInputEl.value;
  if (!displayName || !password) {
    showRegistrationError("Enter a display name and password.");
    return;
  }
  if (displayName.length < 3) {
    showRegistrationError("Display name must be at least 3 characters.");
    return;
  }
  if (password.length < 12) {
    showRegistrationError("Password must be at least 12 characters.");
    return;
  }

  state.authRequestPending = true;
  hideRegistrationError();
  updateAuthControls();

  let response;
  try {
    response = await fetchWithTimeout("/players", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      credentials: "same-origin",
      body: JSON.stringify({ displayName, password })
    });
  } catch (err) {
    state.authRequestPending = false;
    updateAuthControls();
    showRegistrationError(err && err.name === "AbortError" ? "Request timed out. Try again." : "Could not create the account. Check your connection.");
    return;
  }

  const result = response.ok ? await readJSON(response) : null;
  state.authRequestPending = false;
  updateAuthControls();
  if (!response.ok) {
    showRegistrationError(registrationErrorMessage(response.status, await readErrorMessage(response)));
    return;
  }
  if (!result || !result.created || !isPlayer(result.player)) {
    showRegistrationError(registrationErrorMessage(response.status));
    return;
  }

  state.registrationHandle = result.player.accountHandle;
  state.accountView = "registration-success";
  registrationFormEl.reset();
  renderAccountState();
}

async function logout() {
  if (state.authRequestPending) return;

  state.authRequestPending = true;
  updateAuthControls();
  let response;
  try {
    response = await fetchWithTimeout("/auth/logout", {
      method: "POST",
      credentials: "same-origin"
    });
  } catch {
    state.authRequestPending = false;
    updateAuthControls();
    showError("Could not log out. Check your connection.");
    return;
  }

  state.authRequestPending = false;
  if (!response.ok) {
    if (response.status === httpStatusUnauthorized) {
      clearAuthenticatedPlayer();
      showError("Session expired.");
    } else {
      updateAuthControls();
      showError("Could not log out. Try again.");
    }
    return;
  }

  clearAuthenticatedPlayer();
}

function clearAuthenticatedPlayer() {
  state.player = null;
  state.accountView = "guest";
  state.registrationHandle = "";
  state.authStateVersion++;
  renderAccountState();
  syncScoreSubmissionIdentity();
}

function setAuthenticatedPlayer(player) {
  state.player = player;
  state.accountView = "guest";
  state.registrationHandle = "";
  state.authStateVersion++;
  renderAccountState();
  syncScoreSubmissionIdentity();
}

function isPlayer(value) {
  return value &&
    typeof value.displayName === "string" && value.displayName.trim() !== "" &&
    typeof value.accountHandle === "string" && value.accountHandle.trim() !== "";
}

async function readJSON(response) {
  if (!(response.headers.get("Content-Type") || "").toLowerCase().includes("application/json")) {
    return null;
  }
  try {
    return await response.json();
  } catch {
    return null;
  }
}

function loginErrorMessage(status) {
  if (status === httpStatusUnauthorized) return "Account handle or password is incorrect.";
  if (status === 400) return "Enter your account handle and password.";
  if (status === 408) return "Request timed out. Try again.";
  return status >= 500 ? "Server error. Try again." : "Could not log in. Try again.";
}

function registrationErrorMessage(status, errorText) {
  if (status === 400) {
    switch (errorText) {
      case "invalid password: password must be at least 12 characters":
        return "Password must be at least 12 characters.";
      case "invalid password: password must be at most 72 bytes":
        return "Password must be at most 72 bytes.";
      case "invalid password: password must include an ASCII special character":
        return "Password must include an ASCII special character.";
      case "invalid password: password must include an ASCII uppercase letter":
        return "Password must include an ASCII uppercase letter.";
      default:
        return "Check the display name and password.";
    }
  }
  if (status === 408) return "Request timed out. Try again.";
  if (status === 503) return "Could not create the account. Try again.";
  return status >= 500 ? "Server error. Try again." : "Could not create the account. Try again.";
}

async function readErrorMessage(response) {
  try {
    return (await response.text()).trim();
  } catch {
    return "";
  }
}

function showLoginError(message) {
  loginErrorEl.textContent = message;
  loginErrorEl.hidden = false;
}

function hideLoginError() {
  loginErrorEl.textContent = "";
  loginErrorEl.hidden = true;
}

function showRegistrationError(message) {
  registrationErrorEl.textContent = message;
  registrationErrorEl.hidden = false;
}

function hideRegistrationError() {
  registrationErrorEl.textContent = "";
  registrationErrorEl.hidden = true;
}

function getSelectedBoardSize() {
  const checked = document.querySelector('input[name="boardSize"]:checked');
  return checked ? Number(checked.value) : 9;
}

function getSelectedTimer() {
  const checked = document.querySelector('input[name="timer"]:checked');
  return checked ? Number(checked.value) : 120;
}

function getSelectedFont() {
  const checked = document.querySelector('input[name="font"]:checked');
  return checked ? checked.value : "chalk";
}

function getSelectedBoardColor() {
  const active = document.querySelector(".color-swatches .swatch--active");
  const color = active?.dataset.boardColor ?? "green";
  return validBoardColors.has(color) ? color : "green";
}

function applyBoardColor(color) {
  gameScreen.classList.remove("board-color-blue", "board-color-red", "board-color-purple");
  if (color !== "green" && validBoardColors.has(color)) {
    gameScreen.classList.add(`board-color-${color}`);
  }
}

function showGameOverOverlay() {
  gameOverReasonEl.textContent = gameOverReasonText(state.gameOverReason);
  gameOverScoreEl.textContent = state.score;
  updateScoreSubmissionState();
  gameOverOverlay.hidden = false;
}

function hideGameOverOverlay() {
  gameOverOverlay.hidden = true;
}

async function showScoreSubmissionOverlay() {
  if (state.scoreSubmitted) {
    return;
  }

  gameOverOverlay.hidden = true;
  resetScoreSubmissionRequest();
  syncScoreSubmissionIdentity();
  hideScoreSubmitError();
  scoreSubmissionOverlay.hidden = false;
  if (state.player) {
    submitScoreButtonEl.focus();
  } else {
    playerNameInputEl.focus();
  }

  const authenticated = await refreshScoreSubmissionIdentity();
  if (authenticated === false && !scoreSubmissionOverlay.hidden) {
    playerNameInputEl.focus();
  }
}

function hideScoreSubmissionOverlay() {
  scoreSubmissionOverlay.hidden = true;
  resetScoreSubmissionRequest({ abort: true });
}

async function submitScore(event) {
  event.preventDefault();
  const playerName = playerNameInputEl.value.trim();
  const authenticated = state.player !== null;
  if ((!authenticated && !playerName) || state.scoreSubmitted || state.scoreSubmitPending) {
    updateScoreSubmitButton();
    return;
  }
  if (!state.gameId) {
    showScoreSubmitError("Game session is no longer available.");
    return;
  }

  const gameId = state.gameId;
  const controller = new AbortController();
  state.scoreSubmitPending = true;
  state.scoreSubmitGameId = gameId;
  state.scoreSubmitController = controller;
  hideScoreSubmitError();
  updateScoreSubmitButton();

  let response;
  try {
    const body = { gameId };
    if (!authenticated) {
      body.playerName = playerName;
    }
    response = await fetchWithTimeout("/scores", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      credentials: "same-origin",
      body: JSON.stringify(body)
    }, controller);
  } catch (err) {
    if (!scoreSubmissionStillCurrent(gameId)) {
      resetScoreSubmissionFor(gameId);
      return;
    }
    resetScoreSubmissionRequest();
    showScoreSubmitError(err && err.name === "AbortError" ? "Request timed out. Try again." : "Could not submit score. Check your connection.");
    return;
  }

  if (!scoreSubmissionStillCurrent(gameId)) {
    resetScoreSubmissionFor(gameId);
    if (response.status === 201) {
      showError("Score submitted for previous game.");
    }
    return;
  }

  if (response.status === 201) {
    state.scoreSubmitted = true;
    resetScoreSubmissionRequest();
    hideScoreSubmitError();
    hideScoreSubmissionOverlay();
    showGameOverOverlay();
    return;
  }

  resetScoreSubmissionRequest();
  if (response.status === 400 && authenticated) {
    const sessionState = await refreshScoreSubmissionIdentity();
    if (sessionState === false) {
      showScoreSubmitError("Session expired. Enter a player name.");
      return;
    }
    showScoreSubmitError("Could not submit score. Try again.");
    return;
  }
  showScoreSubmitError(scoreSubmissionErrorMessage(response.status));
}

function syncScoreSubmissionIdentity() {
  const authenticated = state.player !== null;
  playerNameLabelEl.textContent = authenticated ? "Account Name" : "Player Name";
  playerNameInputEl.readOnly = authenticated;
  playerNameInputEl.classList.toggle("score-submit-form__input--account", authenticated);
  playerNameInputEl.placeholder = authenticated ? "" : "Your name";
  playerNameInputEl.value = authenticated ? state.player.displayName : "";
  updateScoreSubmitButton();
}

async function refreshScoreSubmissionIdentity() {
  const authStateVersion = state.authStateVersion;
  let response;
  try {
    response = await fetchWithTimeout("/auth/me", { credentials: "same-origin" });
  } catch {
    return null;
  }

  if (authStateVersion !== state.authStateVersion) {
    return null;
  }
  if (response.status === httpStatusUnauthorized) {
    clearAuthenticatedPlayer();
    return false;
  }
  if (!response.ok) {
    return null;
  }

  const result = await readJSON(response);
  if (!result || !result.authenticated || !isPlayer(result.player)) {
    return null;
  }

  setAuthenticatedPlayer(result.player);
  return true;
}

function updateScoreSubmitButton() {
  const hasIdentity = state.player !== null || playerNameInputEl.value.trim() !== "";
  submitScoreButtonEl.disabled = !hasIdentity || state.scoreSubmitted || state.scoreSubmitPending;
  submitScoreButtonEl.textContent = state.scoreSubmitPending ? "Submitting" : "Submit";
  playerNameInputEl.disabled = state.scoreSubmitPending;
  cancelScoreSubmitButtonEl.disabled = false;
}

function updateScoreSubmissionState() {
  submitScoreOpenButtonEl.hidden = state.scoreSubmitted;
  scoreSubmittedStatusEl.hidden = !state.scoreSubmitted;
}

function scoreSubmissionStillCurrent(gameId) {
  return state.gameId === gameId && state.scoreSubmitGameId === gameId;
}

function resetScoreSubmissionFor(gameId) {
  if (state.scoreSubmitGameId === gameId || state.scoreSubmitGameId === null) {
    state.scoreSubmitPending = false;
    state.scoreSubmitGameId = null;
    state.scoreSubmitController = null;
    updateScoreSubmitButton();
  }
}

function resetScoreSubmissionRequest(options = {}) {
  if (options.abort && state.scoreSubmitController) {
    state.scoreSubmitController.abort();
  }
  state.scoreSubmitPending = false;
  state.scoreSubmitGameId = null;
  state.scoreSubmitController = null;
  updateScoreSubmitButton();
}

function showScoreSubmitError(message) {
  scoreSubmitErrorEl.textContent = message;
  scoreSubmitErrorEl.hidden = false;
}

function hideScoreSubmitError() {
  scoreSubmitErrorEl.textContent = "";
  scoreSubmitErrorEl.hidden = true;
}

function scoreSubmissionErrorMessage(status) {
  switch (status) {
    case 400:
      return "Use letters, numbers, spaces, hyphen, underscore, or apostrophe.";
    case 404:
      return "Game session is no longer available.";
    case 408:
      return "Could not submit score. Check your connection.";
    case 409:
      return "Score already submitted or game is not finished.";
    default:
      if (status >= 500) {
        return "Server error. Try submitting again.";
      }
      return "Could not submit score. Try again.";
  }
}

function updateLeaderboardFilter(filter, value) {
  const numericValue = Number(value);
  if (!Number.isInteger(numericValue)) {
    return;
  }

  let changed = false;
  if (filter === "board" && state.leaderboardBoardSize !== numericValue) {
    state.leaderboardBoardSize = numericValue;
    changed = true;
  } else if (filter === "timer" && state.leaderboardDuration !== numericValue) {
    state.leaderboardDuration = numericValue;
    changed = true;
  }

  if (changed) {
    updateLeaderboardFilterSummary();
    if (!leaderboardScreen.hidden) {
      loadLeaderboardScores();
    }
  }
}

async function loadLeaderboardScores() {
  const requestId = ++state.leaderboardRequestId;
  state.leaderboardLoading = true;
  state.leaderboardError = false;
  renderLeaderboardStatus("Loading scores...");

  const gridSize = encodeURIComponent(String(state.leaderboardBoardSize));
  const duration = encodeURIComponent(String(state.leaderboardDuration));
  let response;
  try {
    response = await fetchWithTimeout(`/scores?gridSize=${gridSize}&duration=${duration}`);
  } catch {
    if (requestId !== state.leaderboardRequestId) return;
    state.leaderboardLoading = false;
    state.leaderboardError = true;
    renderLeaderboardStatus("Could not load scores.");
    return;
  }

  if (requestId !== state.leaderboardRequestId) return;

  if (!response.ok) {
    state.leaderboardLoading = false;
    state.leaderboardError = true;
    renderLeaderboardStatus("Could not load scores.");
    return;
  }
  if (!(response.headers.get("Content-Type") || "").toLowerCase().includes("application/json")) {
    state.leaderboardLoading = false;
    state.leaderboardError = true;
    renderLeaderboardStatus("Could not load scores.");
    return;
  }

  let scores;
  try {
    scores = await response.json();
  } catch {
    if (requestId !== state.leaderboardRequestId) return;
    state.leaderboardLoading = false;
    state.leaderboardError = true;
    renderLeaderboardStatus("Could not load scores.");
    return;
  }

  if (requestId !== state.leaderboardRequestId) return;

  if (!Array.isArray(scores)) {
    state.leaderboardLoading = false;
    state.leaderboardError = true;
    renderLeaderboardStatus("Could not load scores.");
    return;
  }

  state.leaderboardScores = scores;
  state.leaderboardLoading = false;
  state.leaderboardError = false;
  renderLeaderboardScores();
}

function renderLeaderboardScores() {
  updateLeaderboardFilterSummary();
  leaderboardBodyEl.replaceChildren();

  if (state.leaderboardScores.length === 0) {
    leaderboardEmptyEl.textContent = "No scores yet - play a game!";
    leaderboardEmptyEl.hidden = false;
    return;
  }

  leaderboardEmptyEl.hidden = true;
  state.leaderboardScores.forEach((score) => {
    const row = document.createElement("tr");
    row.appendChild(newLeaderboardCell(String(score.rank ?? "")));
    row.appendChild(newLeaderboardCell(score.playerName ?? ""));
    row.appendChild(newLeaderboardCell(String(score.score ?? 0)));
    row.appendChild(newLeaderboardCell(formatRemainingMillis(score.remainingMillis)));
    leaderboardBodyEl.appendChild(row);
  });
}

function renderLeaderboardStatus(message) {
  updateLeaderboardFilterSummary();
  leaderboardBodyEl.replaceChildren();
  leaderboardEmptyEl.textContent = message;
  leaderboardEmptyEl.hidden = false;
}

function updateLeaderboardFilterSummary() {
  leaderboardFilterSummaryEl.textContent = `Showing ${state.leaderboardBoardSize} x ${state.leaderboardBoardSize} - ${state.leaderboardDuration}s`;
}

function newLeaderboardCell(text) {
  const cell = document.createElement("td");
  cell.textContent = text;
  return cell;
}

function formatRemainingMillis(value) {
  const millis = Number(value);
  if (!Number.isFinite(millis) || millis <= 0) {
    return "0s";
  }
  if (millis < 1000) {
    return "<1s";
  }
  return `${Math.floor(millis / 1000)}s`;
}

async function fetchWithTimeout(resource, options = {}, controller = new AbortController()) {
  const timeout = setTimeout(() => controller.abort(), scoreRequestTimeoutMs);
  try {
    return await fetch(resource, { ...options, signal: controller.signal });
  } finally {
    clearTimeout(timeout);
  }
}

async function startGame() {
  if (state.startPending) return;
  resetScoreSubmissionRequest({ abort: true });
  state.startPending = true;
  playButtonEl.disabled = true;
  playAgainButtonEl.disabled = true;

  try {
    const gen = ++state.startGeneration;
    const previousGameId = state.gameId;
    const size = getSelectedBoardSize();
    const duration = getSelectedTimer();
    const font = getSelectedFont();
    const boardColor = getSelectedBoardColor();

    closeStream();
    stopCountdown();
    hideGameOverOverlay();
    applyBoardColor(boardColor);

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
        body: JSON.stringify({ size, duration })
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
    state.scoreSubmitted = false;
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
    hideScoreSubmitError();
    updateScoreSubmitButton();

    boardEl.classList.remove("font-chalk", "font-clean", "font-retro");
    boardEl.classList.add(`font-${font}`);

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
    endGame();
    return;
  }
  if (response.status === 422) {
    state.reshuffleUsed = true;
    updateSkillButtons();
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
    state.hintHighlight = null;
    state.hintSnapshotPending = false;
    endGame();
    return;
  }
  if (response.status === 422) {
    state.hintUsed = true;
    updateSkillButtons();
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
    endGame();
    return;
  }
  if (response.status === 422) {
    state.removeNumberUsed = true;
    state.removeMode = false;
    updateSkillButtons();
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
