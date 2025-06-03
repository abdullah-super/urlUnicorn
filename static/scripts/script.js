async function shortenURL() {
  const url = document.getElementById("urlInput").value;
  if (!url) return;

  const response = await fetch("/api/shorten", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ url: url }),
  });

  const data = await response.json();
  document.getElementById("result").innerHTML =
    '<div class="result"><strong>Short URL:</strong><br><a href="' +
    data.short_url +
    '">' +
    data.short_url +
    "</a></div>";
}

async function generateQR() {
  const url = document.getElementById("urlInput").value;
  if (!url) return;

  const response = await fetch("/api/qr", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ url: url }),
  });

  const data = await response.json();
  document.getElementById("qrResult").innerHTML =
    '<div class="result"><strong>QR Code:</strong><br>' +
    '<img id="qrImage" src="/static/' +
    data.qr_file +
    '" alt="QR Code">' +
    "</div>";
}
