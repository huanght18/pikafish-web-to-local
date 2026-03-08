class $1 {
    constructor(e, r) {
        Ge(this, "initEngine", () => {
            this.Ready = !1;
            let e = this.WasmType;
            if (e.indexOf("multi") !== -1)
                this.Mode = "multi",
                this.attachMultiThreadErrorHandlers(),
                U1.import(ya + "/" + e + "/pikafish.js").then( () => {
                    console.log(window.location.origin + ya + "/" + e + "/"),
                    self.Pikafish({
                        onReceiveStdout: this.onReceiveOutput,
                        onExit: this.onExit,
                        locateFile: r => r === "pikafish.data" ? window.location.origin + ya + "/data/" + r : window.location.origin + ya + "/" + e + "/" + r,
                        setStatus: r => {
                            this.DownloadEvent != null && this.DownloadEvent(r)
                        }
                    }).then(r => {
                        this.Engine = r,
                        setTimeout( () => {
                            this.sendCommand("uci"),
                            this.setOptionList(this.EngineOptions)
                        }
                        , 100)
                    }
                    )
                }
                );
            else {
                this.Mode = "single",
                this.detachMultiThreadErrorHandlers();
                let r = new URL("data:application/javascript;base64,dmFyIEVuZ2luZUluc3RhbmNlID0gbnVsbDsKCmNvbnN0IEVOR0lORV9QQVRIID0gJy9lbmdpbmUvbWFpbl8yMDI0MDgxNnY3JzsKCnNlbGYub25tZXNzYWdlID0gZnVuY3Rpb24gKGUpIHsKICAgIGlmIChlLmRhdGEuY29tbWFuZCAhPSBudWxsKSB7CiAgICAgICAgRW5naW5lSW5zdGFuY2Uuc2VuZENvbW1hbmQoZS5kYXRhLmNvbW1hbmQpOwogICAgfSBlbHNlIGlmIChlLmRhdGEud2FzbV90eXBlICE9IG51bGwpIHsKICAgICAgICBsZXQgd2FzbVR5cGUgPSBlLmRhdGEud2FzbV90eXBlOwogICAgICAgIGxldCBvcmlnaW4gPSBlLmRhdGEub3JpZ2luOwogICAgICAgIGNvbnNvbGUubG9nKCJUcnkgdG8gbG9hZDogIiArIG9yaWdpbiArIEVOR0lORV9QQVRIICsgIi8iICsgd2FzbVR5cGUgKyAiL3Bpa2FmaXNoLmpzIikKICAgICAgICBzZWxmLmltcG9ydFNjcmlwdHMob3JpZ2luICsgRU5HSU5FX1BBVEggKyAiLyIgKyB3YXNtVHlwZSArICIvcGlrYWZpc2guanMiKTsKICAgICAgICBzZWxmWydQaWthZmlzaCddKAogICAgICAgICAgICB7CiAgICAgICAgICAgICAgICBvblJlY2VpdmVTdGRvdXQ6IChvKSA9PiBzZWxmLnBvc3RNZXNzYWdlKHsgc3Rkb3V0OiBvIH0pLAogICAgICAgICAgICAgICAgb25FeGl0OiAoYykgPT4gc2VsZi5wb3N0TWVzc2FnZSh7IGV4aXQ6IGMgfSksCiAgICAgICAgICAgICAgICBsb2NhdGVGaWxlOiAodXJsKSA9PiB7CiAgICAgICAgICAgICAgICAgICAgaWYgKHVybCA9PT0gJ3Bpa2FmaXNoLmRhdGEnKSB7CiAgICAgICAgICAgICAgICAgICAgICAgIHJldHVybiBvcmlnaW4gKyBFTkdJTkVfUEFUSCArICIvZGF0YS8iICsgdXJsOwogICAgICAgICAgICAgICAgICAgIH0KICAgICAgICAgICAgICAgICAgICByZXR1cm4gb3JpZ2luICsgRU5HSU5FX1BBVEggKyAiLyIgKyB3YXNtVHlwZSArICIvIiArIHVybDsKICAgICAgICAgICAgICAgIH0sCiAgICAgICAgICAgICAgICBzZXRTdGF0dXM6IChzdGF0dXMpID0+IHsgc2VsZi5wb3N0TWVzc2FnZSh7IGRvd25sb2FkOiBzdGF0dXMgfSk7IH0KICAgICAgICAgICAgfQogICAgICAgICkudGhlbihwID0+IHsKICAgICAgICAgICAgRW5naW5lSW5zdGFuY2UgPSBwOwogICAgICAgICAgICBzZWxmLnBvc3RNZXNzYWdlKHsgcmVhZHk6IHRydWUgfSk7CiAgICAgICAgfSk7CiAgICB9Cn0=",self.location);
                this.Engine = new Worker(r,{
                    type: "classic"
                }),
                this.Engine.onmessage = i => {
                    i.data.ready != null ? setTimeout( () => {
                        this.sendCommand("uci"),
                        this.setOptionList(this.EngineOptions)
                    }
                    , 100) : i.data.stdout != null ? this.onReceiveOutput(i.data.stdout) : i.data.download != null ? this.DownloadEvent != null && this.DownloadEvent(i.data.download) : i.data.exit != null && this.onExit(i.data.exit)
                }
                ,
                this.Engine.postMessage({
                    wasm_type: e,
                    origin: window.location.origin
                })
            }
        }
        );
        Ge(this, "setFen", e => {
            this.AnalyzingFen = e,
            this.sendCommand("fen " + e)
        }
        );
        Ge(this, "sendCommand", e => {
            this.Engine && (console.log(" < " + e),
            this.Mode === "multi" ? this.Engine != null && this.Ready || e === "uci" ? this.Engine.sendCommand(e) : setTimeout( () => {
                this.sendCommand(e)
            }
            , 300) : this.Engine != null && this.Ready || e === "uci" ? this.Engine.postMessage({
                command: e
            }) : setTimeout( () => {
                this.sendCommand(e)
            }
            , 300))
        }
        );
        Ge(this, "setOption", (e, r) => {
            if (e.toLowerCase() === "hash") {
                let i = 256;
                this.Simd && this.Mode === "multi" ? i = 384 : !this.Simd && this.Mode === "single" && (i = 64),
                r > i && (r = i)
            }
            this.sendCommand("setoption name " + e + " value " + r)
        }
        );
        Ge(this, "setOptionList", e => {
            for (let r in e)
                this.setOption(r, e[r])
        }
        );
        Ge(this, "go", (e=-1, r=-1, i=-1, n=null) => {
            if (this.Analyzing)
                return !1;
            {
                this.Analyzing = !0;
                let a = "go";
                return e > 0 && (a += " movetime " + e),
                r > 0 && (a += " depth " + r),
                i > 0 && (a += " nodes " + i),
                n != null && n.length > 0 && (a += " searchmoves " + n.join(" ")),
                this.sendCommand(a),
                !0
            }
        }
        );
        Ge(this, "stop", () => {
            if (this.Mode === "multi")
                this.sendCommand("stop");
            else {
                let e = this.Ready;
                if (this.Ready = !1,
                this.Analyzing = !1,
                this.LastPV != null)
                    try {
                        this.LastPV.length === 2 ? this.BestmoveEvent(this.LastPV[0], this.LastPV[1]) : this.BestmoveEvent(this.LastPV[0], null)
                    } catch (r) {
                        console.error(r)
                    }
                this.Engine != null && e && (console.warn("Terminating engine"),
                this.Engine.terminate(),
                this.Engine = null,
                this.initEngine())
            }
        }
        );
        Ge(this, "onReceiveOutput", e => {
            try {
                if (console.log(" > " + e),
                !e)
                    return;
                let r = e.split(" ")
                  , i = r[0]
                  , n = "depth seldepth time nodes pv multipv score currmove currmovenumber hashfull nps tbhits cpuload string refutation currline".split(" ");
                if (i === "info") {
                    let a = {};
                    for (let o = 1; o < r.length; o++)
                        if (r[o] === "pv") {
                            a[r[o]] = r.slice(o + 1).join(" ");
                            break
                        } else if (r[o] === "string") {
                            a[r[o]] = r.slice(o + 1).join(" ");
                            break
                        } else
                            r[o] === "score" ? r[o + 1] === "cp" ? (a[r[o]] = {
                                cp: parseInt(r[o + 2])
                            },
                            o += 2) : r[o + 1] === "mate" ? (a[r[o]] = {
                                mate: parseInt(r[o + 2])
                            },
                            o += 2) : (a[r[o]] = {
                                cp: parseInt(r[o + 1])
                            },
                            o++) : r[o] === "wdl" ? r.slice(o + 1, o + 4).every(s => !isNaN(s)) && (a[r[o]] = r.slice(o + 1, o + 4).map(s => parseInt(s)),
                            o += 3) : r.length > o + 1 && n.includes(r[o]) ? (a[r[o]] = parseInt(r[o + 1]),
                            o++) : a[r[o]] = "";
                    "pv"in a && (this.LastPV = a.pv.split(" ")),
                    this.InfoEvent != null && this.InfoEvent(i, a)
                } else
                    i === "bestmove" ? (this.Analyzing = !1,
                    r.length === 4 && r[2] === "ponder" ? this.BestmoveEvent != null && this.BestmoveEvent(r[1], r[3]) : this.BestmoveEvent != null && this.BestmoveEvent(r[1], null)) : i === "option" || i === "uciok" && (this.UCIOKEvent != null && this.UCIOKEvent(),
                    this.Ready = !0)
            } catch (r) {
                console.error(r)
            }
        }
        );
        Ge(this, "onExit", e => {
            console.error("Engine exited with code " + e),
            this.ExitEvent != null && this.ExitEvent(e),
            this.Engine = null,
            this.initEngine()
        }
        );
        Ge(this, "isAppleSafari", () => {
            if (typeof navigator == "undefined")
                return !1;
            const e = navigator.userAgent || ""
              , r = /Safari/.test(e) && !/(Chrome|Chromium|Edg|OPR|CriOS|FxiOS)/.test(e);
            return /AppleWebKit/.test(e) && r
        }
        );
        Ge(this, "getSingleWasmType", e => e ? e.includes("multi_simd_relaxed") || e.includes("multi_simd") ? "single_simd" : e.includes("multi") ? "single" : e : "single");
        Ge(this, "attachMultiThreadErrorHandlers", () => {
            !this.isAppleSafari() || this._multiErrorHandler || this._multiRejectionHandler || (this._multiErrorHandler = e => {
                var i;
                const r = ((i = e == null ? void 0 : e.error) == null ? void 0 : i.message) || (e == null ? void 0 : e.message) || "";
                r.includes("Out of bounds memory access") && this.fallbackToSingleThread(r)
            }
            ,
            this._multiRejectionHandler = e => {
                var i;
                const r = ((i = e == null ? void 0 : e.reason) == null ? void 0 : i.message) || String((e == null ? void 0 : e.reason) || "");
                r.includes("Out of bounds memory access") && this.fallbackToSingleThread(r)
            }
            ,
            window.addEventListener("error", this._multiErrorHandler),
            window.addEventListener("unhandledrejection", this._multiRejectionHandler))
        }
        );
        Ge(this, "detachMultiThreadErrorHandlers", () => {
            this._multiErrorHandler && (window.removeEventListener("error", this._multiErrorHandler),
            this._multiErrorHandler = null),
            this._multiRejectionHandler && (window.removeEventListener("unhandledrejection", this._multiRejectionHandler),
            this._multiRejectionHandler = null)
        }
        );
        Ge(this, "fallbackToSingleThread", e => {
            if (!(this.FallbackTriggered || this.Mode !== "multi")) {
                this.FallbackTriggered = !0,
                console.warn("Multi-threaded engine failed, fallback to single.", e);
                try {
                    this.Engine && typeof this.Engine.terminate == "function" && this.Engine.terminate()
                } catch (r) {
                    console.warn("Engine terminate failed.", r)
                }
                this.Engine = null,
                this.Ready = !1,
                this.Analyzing = !1,
                this.Mode = "single",
                this.WasmType = this.getSingleWasmType(this.WasmType),
                this.detachMultiThreadErrorHandlers(),
                this.initEngine()
            }
        }
        );
        this.Engine = null,
        this.InfoEvent = null,
        this.BestmoveEvent = null,
        this.ExitEvent = null,
        this.OptionEvent = null,
        this.StatusEvent = null,
        this.DownloadEvent = null,
        this.UCIOKEvent = null,
        this.Ready = !1,
        this.Analyzing = !1,
        this.EngineOptions = r,
        this.Mode = "single",
        this.Simd = !1,
        this.LastPV = null,
        this.WasmType = null,
        this.EngineType = e,
        this.AnalyzingFen = "",
        this.endOfAnalysis = !1,
        this.FallbackTriggered = !1,
        this._multiErrorHandler = null,
        this._multiRejectionHandler = null,
        H1().then(i => {
            this.WasmType = i,
            i.includes("simd") && (this.Simd = !0),
            this.initEngine()
        }
        )
    }
}