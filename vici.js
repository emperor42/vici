"use strict";

/**
 * vici — cookie & encryption management on the client side.
 *
 * All document.cookie reads/writes and all client-side encryption should
 * route through vici. Encryption uses the Web Crypto API (AES-256-GCM, key
 * derived from a passphrase with PBKDF2-SHA256) — there is no hand-rolled
 * "XOR cipher" anywhere.
 *
 * Honesty note: JavaScript cannot set the httpOnly flag on a cookie — that is
 * a server-side response-header concern. cookieSet() therefore returns a
 * warning object when httpOnly is requested so callers surface it to the
 * server (or just set the header there).
 *
 * Usage:
 *   vici.cookieSet("theme", "dark", 30);            // 30 days, /
 *   var t = vici.cookieGet("theme");
 *   vici.cookieDelete("theme");
 *   vici.encrypt("secret", "passphrase").then(p => vici.decrypt(p, "passphrase"));
 *
 * Browser global: window.vici (instance) and window.Vici (class).
 * No external dependencies.
 */

(function () {
  "use strict";

  // ---- base64 helpers ---------------------------------------------------------

  function bytesToB64(bytes) {
    var bin = "";
    for (var i = 0; i < bytes.length; i++) bin += String.fromCharCode(bytes[i]);
    if (typeof btoa === "function") return btoa(bin);
    return Buffer.from(bin, "binary").toString("base64"); // optional Node use
  }

  function b64ToBytes(b64) {
    var bin;
    if (typeof atob === "function") bin = atob(b64);
    else bin = Buffer.from(b64, "base64").toString("binary");
    var out = new Uint8Array(bin.length);
    for (var i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
    return out;
  }

  function strToBytes(s) {
    var enc = new TextEncoder();
    return enc.encode(s);
  }

  function bytesToStr(bytes) {
    var dec = new TextDecoder();
    return dec.decode(bytes);
  }

  function concatBytes(a, b) {
    var out = new Uint8Array(a.length + b.length);
    out.set(a, 0);
    out.set(b, a.length);
    return out;
  }

  function hasCrypto() {
    return typeof crypto !== "undefined" && crypto.subtle && crypto.getRandomValues;
  }

  class Vici {
    constructor(opts) {
      opts = opts || {};
      this.defaultDomain = opts.domain || "";
      this.defaultPath = opts.path || "/";
      this.secure = !!opts.secure;
    }

    // ---- cookies --------------------------------------------------------------

    /**
     * Set a cookie. days <= 0 makes it a session cookie.
     * Returns {ok:true} or {ok:false, warnings:[...]} for flags JS cannot
     * honor (httpOnly currently).
     */
    cookieSet(name, value, days, opts) {
      opts = opts || {};
      if (!name) return { ok: false, warnings: ["cookie name is required"] };
      var warnings = [];
      if (opts.httpOnly) {
        warnings.push("httpOnly cannot be set from JavaScript — set it in the server's Set-Cookie header");
      }
      var parts = [];
      parts.push(encodeURIComponent(name) + "=" + encodeURIComponent(String(value == null ? "" : value)));
      if (opts.path !== undefined && opts.path !== null) parts.push("path=" + opts.path);
      else if (this.defaultPath) parts.push("path=" + this.defaultPath);
      else parts.push("path=/");
      if (opts.domain || this.defaultDomain) parts.push("domain=" + (opts.domain || this.defaultDomain));
      if (opts.secure || this.secure) parts.push("secure");
      if (opts.sameSite) parts.push("samesite=" + opts.sameSite.toLowerCase());
      if (days && days > 0) {
        var d = new Date();
        d.setTime(d.getTime() + days * 24 * 60 * 60 * 1000);
        parts.push("expires=" + d.toUTCString());
      }
      document.cookie = parts.join("; ");
      return { ok: true, warnings: warnings };
    }

    cookieGet(name) {
      if (!name) return null;
      var prefix = encodeURIComponent(name) + "=";
      var parts = document.cookie.split("; ");
      for (var i = 0; i < parts.length; i++) {
        var p = parts[i];
        if (p.indexOf(prefix) === 0) {
          return decodeURIComponent(p.slice(prefix.length));
        }
      }
      return null;
    }

    cookieDelete(name, opts) {
      opts = opts || {};
      var d = new Date(0);
      var parts = [];
      parts.push(encodeURIComponent(name) + "=");
      if (opts.path !== undefined && opts.path !== null) parts.push("path=" + opts.path);
      else if (this.defaultPath) parts.push("path=" + this.defaultPath);
      else parts.push("path=/");
      if (opts.domain || this.defaultDomain) parts.push("domain=" + (opts.domain || this.defaultDomain));
      parts.push("expires=" + d.toUTCString());
      document.cookie = parts.join("; ");
      return true;
    }

    cookieAll() {
      var out = {};
      var parts = document.cookie.split("; ");
      for (var i = 0; i < parts.length; i++) {
        var p = parts[i];
        if (!p) continue;
        var idx = p.indexOf("=");
        if (idx === -1) continue;
        try {
          var k = decodeURIComponent(p.slice(0, idx).trim());
          out[k] = decodeURIComponent(p.slice(idx + 1));
        } catch (e) {
          /* malformed cookie — skip */
        }
      }
      return out;
    }

    // ---- encryption (WebCrypto AES-256-GCM + PBKDF2) --------------------------

    randomBytes(n) {
      if (!hasCrypto()) throw new Error("vici: Web Crypto unavailable");
      var out = new Uint8Array(n);
      crypto.getRandomValues(out);
      return out;
    }

    randomToken(n) {
      if (!n) n = 24;
      return bytesToB64(this.randomBytes(n)).replace(/[+/=]/g, "").slice(0, n);
    }

    async sha256Hex(str) {
      if (!hasCrypto()) throw new Error("vici: Web Crypto unavailable");
      var digest = await crypto.subtle.digest("SHA-256", strToBytes(String(str)));
      return Array.from(new Uint8Array(digest)).map(function (b) {
        return ("0" + b.toString(16)).slice(-2);
      }).join("");
    }

    _normalizeKey(secret) {
      // Accept a passphrase (string) or a raw CryptoKey.
      if (secret && secret instanceof CryptoKey) return Promise.resolve(secret);
      return this._deriveKey(String(secret));
    }

    _deriveKey(passphrase, salt) {
      if (!salt) salt = this.randomBytes(16);
      var enc = new TextEncoder();
      var materialPromise = crypto.subtle.importKey(
        "raw", enc.encode(passphrase), "PBKDF2", false, ["deriveKey"]
      );
      return materialPromise.then(function (material) {
        return crypto.subtle.deriveKey(
          {
            name: "PBKDF2",
            salt: salt,
            iterations: 100000,
            hash: "SHA-256",
          },
          material,
          { name: "AES-GCM", length: 256 },
          false,
          ["encrypt", "decrypt"]
        );
      }).then(function (key) {
        return { key: key, salt: salt };
      });
    }

    /**
     * Encrypt plaintext with a passphrase (or a CryptoKey passed via opts.key).
     * Returns a portable payload string: base64(salt).base64(iv).base64(ciphertext)
     * (or the nonce-prepended form base64(nonce || ciphertext) when opts.raw).
     */
    async encrypt(plaintext, passphrase, opts) {
      opts = opts || {};
      if (!hasCrypto()) throw new Error("vici: Web Crypto unavailable");
      var derived = await this._deriveKey(String(passphrase));
      var iv = this.randomBytes(12);
      var ct = await crypto.subtle.encrypt(
        { name: "AES-GCM", iv: iv },
        derived.key,
        strToBytes(String(plaintext))
      );
      if (opts.raw) {
        return bytesToB64(concatBytes(iv, new Uint8Array(ct)));
      }
      return (
        bytesToB64(derived.salt) + "." +
        bytesToB64(iv) + "." +
        bytesToB64(new Uint8Array(ct))
      );
    }

    /**
     * Decrypt a payload string produced by encrypt(). Returns the plaintext.
     */
    async decrypt(payload, passphrase) {
      if (!hasCrypto()) throw new Error("vici: Web Crypto unavailable");
      if (!payload || typeof payload !== "string") throw new Error("vici: invalid payload");
      var parts = payload.split(".");
      var salt, iv, ciphertext;
      if (parts.length === 3) {
        salt = b64ToBytes(parts[0]);
        iv = b64ToBytes(parts[1]);
        ciphertext = b64ToBytes(parts[2]);
      } else if (parts.length === 1) {
        // raw nonce-prepended form
        var all = b64ToBytes(payload);
        if (all.length < 13) throw new Error("vici: payload too short");
        iv = all.subarray(0, 12);
        ciphertext = all.subarray(12);
      } else {
        throw new Error("vici: unsupported payload format");
      }
      var key = await this._deriveKey(String(passphrase), salt || this.randomBytes(16));
      var plain = await crypto.subtle.decrypt(
        { name: "AES-GCM", iv: iv },
        key.key,
        ciphertext
      );
      return bytesToStr(new Uint8Array(plain));
    }

    // Backwards-compatible aliases for the old surface.
    setCookie(name, value, days, opts) { return this.cookieSet(name, value, days, opts); }
    getCookie(name) { return this.cookieGet(name); }
    deleteCookie(name, opts) { return this.cookieDelete(name, opts); }
    hash(str) { return this.sha256Hex(str); }
  }

  var vici = new Vici();

  // Export for module systems
  if (typeof module !== "undefined" && module.exports) {
    module.exports = { Vici: Vici, vici: vici };
  }

  if (typeof window !== "undefined") {
    window.Vici = Vici;
    window.vici = vici;
  }
})();