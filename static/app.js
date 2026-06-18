var editId = null;
var activeKind = "all";

var kindColours = {
	Event: "#c62828",
	Faction: "#ef6c00",
	Nation: "#1565c0",
	Species: "#2e7d32",
	Leader: "#7b1fa2"
};

function populateDatalists() {
	fetch("/api/cards").then(function(res) { return res.json(); }).then(function(cards) {
		var factions = {}, nations = {}, species = {};
		cards.forEach(function(c) {
			if (c.faction) factions[c.faction] = true;
			if (c.nation) nations[c.nation] = true;
			if (c.species) species[c.species] = true;
		});
		function setOptions(id, values) {
			var dl = document.getElementById(id);
			if (!dl) return;
			dl.innerHTML = "";
			Object.keys(values).sort().forEach(function(v) {
				var opt = document.createElement("option");
				opt.value = v;
				dl.appendChild(opt);
			});
		}
		setOptions("faction-list", factions);
		setOptions("nation-list", nations);
		setOptions("species-list", species);
		setOptions("edit-faction-list", factions);
		setOptions("edit-nation-list", nations);
		setOptions("edit-species-list", species);
	});
}

function applyKindColours() {
	document.querySelectorAll(".card[data-kind]").forEach(function(card) {
		var kind = card.getAttribute("data-kind");
		var clr = kindColours[kind] || "#999";
		card.style.borderLeft = "4px solid " + clr;
	});
}

function filterCards() {
	var q = document.getElementById("search-input").value.toLowerCase();
	var cards = document.querySelectorAll(".card-list > .card");
	var empty = document.querySelector(".card-list > .empty");
	var visible = 0;
	cards.forEach(function(card) {
		var text = card.textContent.toLowerCase();
		var kind = card.getAttribute("data-kind");
		var matchKind = activeKind === "all" || kind === activeKind;
		var matchSearch = text.indexOf(q) > -1;
		var show = matchKind && matchSearch;
		card.style.display = show ? "" : "none";
		if (show) visible++;
	});
	if (empty) empty.style.display = visible > 0 ? "none" : "";
}

document.getElementById("search-input").addEventListener("input", filterCards);

document.getElementById("kind-filters").addEventListener("click", function(e) {
	var btn = e.target.closest(".kind-btn");
	if (!btn) return;
	document.querySelectorAll(".kind-btn").forEach(function(b) { b.classList.remove("active"); });
	btn.classList.add("active");
	activeKind = btn.getAttribute("data-kind");
	filterCards();
});

document.getElementById("timeline").addEventListener("click", function(e) {
	var card = e.target.closest(".card");
	if (!card) return;
	var id = card.getAttribute("data-id");
	openEditModal(id);
});

function openEditModal(id) {
	editId = id;
	fetch("/api/cards/" + id).then(function(res) { return res.json(); }).then(function(c) {
		document.getElementById("edit-name").value = c.name || "";
		document.getElementById("edit-kind").value = c.kind || "Event";
		document.getElementById("edit-faction").value = c.faction || "";
		document.getElementById("edit-nation").value = c.nation || "";
		document.getElementById("edit-species").value = c.species || "";
		document.getElementById("edit-text").value = c.text || "";
		document.getElementById("edit-modal").classList.add("active");
	});
}

function closeEditModal() {
	document.getElementById("edit-modal").classList.remove("active");
	editId = null;
}

document.getElementById("edit-cancel").addEventListener("click", closeEditModal);
document.getElementById("edit-modal").addEventListener("click", function(e) {
	if (e.target.classList.contains("modal-overlay")) closeEditModal();
});

document.getElementById("edit-form").addEventListener("submit", async function(e) {
	e.preventDefault();
	if (!editId) return;
	var card = {
		name: document.getElementById("edit-name").value,
		kind: document.getElementById("edit-kind").value,
		faction: document.getElementById("edit-faction").value,
		nation: document.getElementById("edit-nation").value,
		species: document.getElementById("edit-species").value,
		text: document.getElementById("edit-text").value
	};
	var res = await fetch("/api/cards/" + editId, {
		method: "PUT",
		headers: { "Content-Type": "application/json" },
		body: JSON.stringify(card)
	});
	if (res.ok) window.location.reload();
	else alert("Failed to update card");
});

document.getElementById("card-form").addEventListener("submit", async function(e) {
	e.preventDefault();
	var card = {
		name: document.getElementById("card-name").value,
		kind: document.getElementById("card-kind").value,
		faction: document.getElementById("card-faction").value,
		nation: document.getElementById("card-nation").value,
		species: document.getElementById("card-species").value,
		text: document.getElementById("card-text").value
	};
	var res = await fetch("/api/cards", {
		method: "POST",
		headers: { "Content-Type": "application/json" },
		body: JSON.stringify(card)
	});
	if (res.ok) window.location.reload();
	else alert("Failed to create card");
});

document.getElementById("export-btn").addEventListener("click", function() {
	window.location.href = "/api/export";
});

document.getElementById("import-btn").addEventListener("click", function(e) {
	e.preventDefault();
	document.getElementById("import-file").click();
});

document.getElementById("import-file").addEventListener("change", function() {
	if (this.files.length > 0) {
		var form = new FormData();
		form.append("file", this.files[0]);
		fetch("/api/import", { method: "POST", body: form })
			.then(function(res) { window.location.reload(); });
	}
});

document.getElementById("clear-btn").addEventListener("click", async function() {
	if (confirm("Delete all cards?")) {
		await fetch("/api/cards", { method: "DELETE" });
		window.location.reload();
	}
});

populateDatalists();
applyKindColours();
