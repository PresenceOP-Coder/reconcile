import { useState, useRef } from 'react'
import { marked } from 'marked'
import {
  Upload, Sparkles, CheckCircle2, XCircle, ArrowRight, Activity, Zap, Play, FileText, ChevronDown, ChevronUp, Loader2
} from 'lucide-react'

export default function App() {
  const [files, setFiles] = useState({ gateway: null, bank: null, ledger: null })
  const [config, setConfig] = useState({ amtTol: 1.5, dateWin: 3, minConf: 0.75 })
  const [status, setStatus] = useState('ready') // ready, running, done
  const [statusText, setStatusText] = useState('ready')
  const [loading, setLoading] = useState(false)
  const [results, setResults] = useState(null)
  
  // AI State
  const [aiTab, setAiTab] = useState('explain')
  const [explainId, setExplainId] = useState('')
  const [aiResponse, setAiResponse] = useState(null)
  const [aiLoading, setAiLoading] = useState(false)
  const [reportHTML, setReportHTML] = useState(null)
  
  // Table State
  const [activeFilter, setActiveFilter] = useState('ALL')
  const [searchQuery, setSearchQuery] = useState('')
  const [matchesOpen, setMatchesOpen] = useState(false)

  // Drag and drop handling
  const handleDrop = (e, source) => {
    e.preventDefault()
    e.currentTarget.classList.remove('drag-over')
    const f = e.dataTransfer.files[0]
    if (f) setFiles(prev => ({ ...prev, [source]: f }))
  }
  
  const handleFile = (e, source) => {
    const f = e.target.files[0]
    if (f) setFiles(prev => ({ ...prev, [source]: f }))
  }

  const runReconcile = async () => {
    setStatus('running')
    setStatusText('running...')
    setLoading(true)

    const fd = new FormData()
    Object.entries(files).forEach(([k, v]) => { if (v) fd.append(k, v) })
    fd.append('amount_tolerance_pct', config.amtTol)
    fd.append('date_window_days', config.dateWin)
    fd.append('min_confidence', config.minConf)

    try {
      const res = await fetch('/api/reconcile', { method: 'POST', body: fd })
      const data = await res.json()
      if (!data.success) throw new Error(data.error || 'unknown error')
      setResults(data)
      setStatus('done')
      setStatusText('done')
      window.scrollTo({ top: 0, behavior: 'smooth' })
    } catch (err) {
      alert('Reconciliation failed: ' + err.message)
      setStatus('ready')
      setStatusText('ready')
    } finally {
      setLoading(false)
    }
  }

  const runSample = async () => {
    setStatus('running')
    setStatusText('running...')
    setLoading(true)
    try {
      const res = await fetch('/api/sample', { method: 'POST' })
      const data = await res.json()
      if (!data.success) throw new Error(data.error || 'unknown error')
      setResults(data)
      setStatus('done')
      setStatusText('done')
      window.scrollTo({ top: 0, behavior: 'smooth' })
    } catch (err) {
      alert('Failed: ' + err.message)
      setStatus('ready')
      setStatusText('ready')
    } finally {
      setLoading(false)
    }
  }

  const askAi = async (idToAsk) => {
    const id = idToAsk || explainId
    if (!id) return
    setExplainId(id)
    setAiTab('explain')
    setAiLoading(true)
    setAiResponse(null)
    try {
      const res = await fetch('/api/explain', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ record_id: id })
      })
      const data = await res.json()
      if (data.error) throw new Error(data.error)
      setAiResponse(marked.parse(data.explanation))
    } catch (err) {
      setAiResponse(`**Error**: ${err.message}`)
    } finally {
      setAiLoading(false)
    }
  }

  const generateReport = async () => {
    setAiLoading(true)
    setReportHTML(null)
    try {
      const res = await fetch('/api/agent', { method: 'POST' })
      const data = await res.json()
      if (data.error) throw new Error(data.error)
      setReportHTML(marked.parse(data.report))
    } catch (err) {
      setReportHTML(`**Error**: ${err.message}`)
    } finally {
      setAiLoading(false)
    }
  }

  const reset = () => {
    setResults(null)
    setStatus('ready')
    setStatusText('ready')
    setAiResponse(null)
    setReportHTML(null)
    setFiles({ gateway: null, bank: null, ledger: null })
  }

  // Derived state
  const isReadyToRun = Object.values(files).filter(Boolean).length >= 2
  const exceptions = results?.exceptions || []
  const filteredExceptions = exceptions.filter(e => {
    const matchFilter = activeFilter === 'ALL' || e.reason_code === activeFilter
    const matchSearch = !searchQuery || 
      e.record_id.toLowerCase().includes(searchQuery.toLowerCase()) ||
      e.ref_id.toLowerCase().includes(searchQuery.toLowerCase()) ||
      e.source.toLowerCase().includes(searchQuery.toLowerCase())
    return matchFilter && matchSearch
  })

  return (
    <>
      <header>
        <div className="logo">
          <div className="logo-mark">R</div>
          reconcile
        </div>
        <div className="header-meta">AI Finance Controller</div>
        <div className="status-pill">
          <div className={`status-dot ${status}`}></div>
          <span>{statusText}</span>
        </div>
      </header>

      <main>
        {!results ? (
          <section id="uploadSection">
            <div className="upload-hero">
              <h1>Multi-source Financial Reconciliation</h1>
              <p>Upload your source CSV files, configure tolerances, and run the engine.</p>
            </div>

            <button className="btn btn-sample" onClick={runSample} disabled={loading}>
              <Play size={14} /> Use Built-in Sample Data (74 records)
            </button>

            <div className="drop-grid">
              {['gateway', 'bank', 'ledger'].map(source => (
                <div 
                  key={source}
                  className={`drop-zone ${files[source] ? 'loaded' : ''}`}
                  onDragOver={e => { e.preventDefault(); e.currentTarget.classList.add('drag-over') }}
                  onDragLeave={e => e.currentTarget.classList.remove('drag-over')}
                  onDrop={e => handleDrop(e, source)}
                >
                  <input type="file" accept=".csv" onChange={e => handleFile(e, source)} />
                  <div className="drop-icon">
                    {files[source] ? <CheckCircle2 size={16} /> : <Upload size={16} />}
                  </div>
                  <div className="drop-label">{source} CSV</div>
                  <div className="drop-hint">{files[source] ? '✓ ready' : `${source}_file.csv`}</div>
                  {files[source] && <div className="drop-filename">{files[source].name}</div>}
                </div>
              ))}
            </div>

            <div className="config-panel">
              <div className="config-title">Matching Configuration</div>
              <div className="config-grid">
                <div className="config-item">
                  <label>Amount Tolerance</label>
                  <div className="slider-row">
                    <input type="range" min="0" max="10" step="0.5" value={config.amtTol} onChange={e => setConfig({...config, amtTol: e.target.value})} />
                    <span className="slider-val">{config.amtTol}%</span>
                  </div>
                </div>
                <div className="config-item">
                  <label>Date Window (days)</label>
                  <div className="slider-row">
                    <input type="range" min="0" max="14" step="1" value={config.dateWin} onChange={e => setConfig({...config, dateWin: e.target.value})} />
                    <span className="slider-val">{config.dateWin}d</span>
                  </div>
                </div>
                <div className="config-item">
                  <label>Min Confidence</label>
                  <div className="slider-row">
                    <input type="range" min="0.5" max="1" step="0.05" value={config.minConf} onChange={e => setConfig({...config, minConf: e.target.value})} />
                    <span className="slider-val">{config.minConf}</span>
                  </div>
                </div>
              </div>
            </div>

            <button className="btn btn-primary btn-run" onClick={runReconcile} disabled={!isReadyToRun || loading}>
              {loading ? <Loader2 size={16} className="spin" /> : <Play size={16} />}
              Run Reconciliation
            </button>
          </section>
        ) : (
          <section id="resultsSection">
            <div className="results-header">
              <div className="results-title">Reconciliation Report</div>
              <div className="flex items-center gap-12">
                <span className="elapsed-tag">{results.elapsed_ms.toFixed(2)} ms</span>
                <button className="btn btn-sm" onClick={reset}>
                  <Upload size={13} /> New Run
                </button>
              </div>
            </div>

            <div className="stats-grid">
              <div className="stat-card"><div className="stat-label">Total Records</div><div className="stat-value mono">{results.total_records}</div></div>
              <div className="stat-card"><div className="stat-label">Match Rate</div><div className="stat-value success mono">{results.match_rate_pct.toFixed(2)}%</div></div>
              <div className="stat-card"><div className="stat-label">Exact Matches</div><div className="stat-value mono">{results.exact_matches}</div></div>
              <div className="stat-card"><div className="stat-label">Fuzzy Matches</div><div className="stat-value warning mono">{results.fuzzy_matches}</div></div>
              <div className="stat-card"><div className="stat-label">Exceptions</div><div className="stat-value error mono">{results.exception_count}</div></div>
              <div className="stat-card">
                <div className="stat-label">FP Risk</div>
                <div className="stat-value warning mono">{results.fp_risk}</div>
                <div className="stat-sub">{results.fp_rate_pct.toFixed(2)}% of total</div>
              </div>
            </div>

            <div className={`invariant-bar ${results.invariant_valid ? 'ok' : 'fail'}`}>
              {results.invariant_valid ? <CheckCircle2 size={16} className="invariant-icon"/> : <XCircle size={16} className="invariant-icon"/>}
              <span>
                {results.invariant_valid 
                  ? `Integrity Verified — ${results.exact_matches + results.fuzzy_matches} matched + ${results.exception_count} exceptions = ${results.total_records} total`
                  : 'Integrity Violated — counts do not add up'}
              </span>
            </div>

            <div className="two-col">
              <div className="paper-card">
                <div className="card-title">Exception Breakdown</div>
                {Object.entries(results.exception_breakdown).sort((a,b)=>b[1]-a[1]).map(([k,v]) => (
                  <div className="breakdown-row" key={k}>
                    <div className="breakdown-key">{k}</div>
                    <div className="bar-track">
                      <div className="bar-fill error" style={{width: `${(v / results.exception_count) * 100}%`}}></div>
                    </div>
                    <div className="breakdown-count">{v}</div>
                  </div>
                ))}
              </div>
              <div className="paper-card">
                <div className="card-title">Records by Source</div>
                {Object.entries(results.records_by_source).sort((a,b)=>b[1]-a[1]).map(([k,v]) => (
                  <div className="breakdown-row" key={k}>
                    <div className="breakdown-key">{k}</div>
                    <div className="bar-track">
                      <div className="bar-fill source" style={{width: `${(v / results.total_records) * 100}%`}}></div>
                    </div>
                    <div className="breakdown-count">{v}</div>
                  </div>
                ))}
              </div>
            </div>

            {results.alerts && results.alerts.length > 0 && (
              <div className="alerts-section">
                <div className="card-title mb-0" style={{marginBottom: 12}}>
                  <Activity size={13} style={{display:'inline', verticalAlign:'-2px', marginRight:4}}/>
                  Systemic Alerts
                </div>
                {results.alerts.map((a, i) => (
                  <div className="alert-card" key={i}>
                    <div className="alert-header">
                      <span className="alert-badge">{a.reason_code}</span>
                      <span className="alert-badge" style={{background: 'var(--muted)'}}>{a.source}</span>
                      <span className="alert-count">count: {a.count} (threshold: {a.threshold})</span>
                    </div>
                    <div className="alert-msg">{a.message}</div>
                  </div>
                ))}
              </div>
            )}

            <div className="table-section paper-card mb-0" style={{padding:0, overflow:'hidden', marginBottom:16}}>
              <div style={{padding: '16px 20px 0'}}>
                <div className="card-title" style={{marginBottom:12}}>Exceptions</div>
                <div className="table-controls">
                  {['ALL', 'AMOUNT_MISMATCH', 'DATE_DRIFT', 'DUPLICATE_REF', 'NO_COUNTERPART', 'MALFORMED_INPUT'].map(f => (
                    <button key={f} className={`filter-pill ${activeFilter === f ? 'active' : ''}`} onClick={() => setActiveFilter(f)}>
                      {f.replace(/_/g, ' ')}
                    </button>
                  ))}
                  <input type="text" className="search-input" placeholder="Search records..." value={searchQuery} onChange={e => setSearchQuery(e.target.value)} />
                </div>
              </div>
              <div style={{overflowX: 'auto'}}>
                <table className="data-table">
                  <thead>
                    <tr>
                      <th>Record ID</th>
                      <th>Source</th>
                      <th>Ref ID</th>
                      <th>Amount</th>
                      <th>Reason</th>
                      <th>Detail</th>
                      <th></th>
                    </tr>
                  </thead>
                  <tbody>
                    {filteredExceptions.map(e => (
                      <tr key={e.record_id} className="exc-row" onClick={() => askAi(e.record_id)}>
                        <td><span className="mono">{e.record_id}</span></td>
                        <td><span className="mono text-muted">{e.source}</span></td>
                        <td><span className="mono">{e.ref_id}</span></td>
                        <td><span className="mono">{e.amount.toFixed(2)} <span className="text-muted">{e.currency}</span></span></td>
                        <td><span className={`reason-badge ${e.reason_code}`}>{e.reason_code.replace(/_/g,' ')}</span></td>
                        <td style={{fontSize:12, color:'var(--muted)', maxWidth:220, whiteSpace:'nowrap', overflow:'hidden', textOverflow:'ellipsis'}}>{e.detail}</td>
                        <td><button className="btn btn-sm" style={{padding:'4px 8px', fontSize:11}} onClick={ev => {ev.stopPropagation(); askAi(e.record_id)}}>Ask AI</button></td>
                      </tr>
                    ))}
                    {filteredExceptions.length === 0 && (
                      <tr><td colSpan="7"><div className="empty-state"><p>No exceptions match the current filter.</p></div></td></tr>
                    )}
                  </tbody>
                </table>
              </div>
            </div>

            <div className="collapse-section">
              <button className="collapse-toggle" onClick={() => setMatchesOpen(!matchesOpen)}>
                <span>Matched Records</span>
                {matchesOpen ? <ChevronUp size={14}/> : <ChevronDown size={14}/>}
              </button>
              {matchesOpen && (
                <div className="collapse-body open">
                  <div style={{overflowX: 'auto'}}>
                    <table className="data-table">
                      <thead>
                        <tr>
                          <th>Match ID</th>
                          <th>Ref ID</th>
                          <th>Pass</th>
                          <th>Sources</th>
                          <th>Records</th>
                          <th>Confidence</th>
                        </tr>
                      </thead>
                      <tbody>
                        {results.matches.map(m => (
                          <tr key={m.match_id}>
                            <td><span className="mono" style={{fontSize:11}}>{m.match_id}</span></td>
                            <td><span className="mono">{m.ref_id}</span></td>
                            <td><span className={`pass-badge ${m.pass}`}>{m.pass}</span></td>
                            <td><span className="mono text-muted" style={{fontSize:11}}>{m.sources.join(' · ')}</span></td>
                            <td><span className="mono text-muted">{m.record_count}</span></td>
                            <td>
                              <div className="conf-bar">
                                <div className="conf-track"><div className="conf-fill" style={{width: `${m.confidence * 100}%`}}></div></div>
                                <span className="mono" style={{fontSize:11, color:'var(--muted)'}}>{m.confidence.toFixed(3)}</span>
                              </div>
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </div>
              )}
            </div>

            <div className="ai-panel">
              <div className="ai-panel-header">
                <Sparkles size={15}/>
                <span className="ai-panel-title">AI Finance Controller</span>
                <span style={{marginLeft:'auto', fontFamily:'var(--mono)', fontSize:10, opacity:.6}}>Powered by Gemini</span>
              </div>
              <div className="ai-tabs">
                <button className={`ai-tab ${aiTab === 'explain' ? 'active' : ''}`} onClick={() => setAiTab('explain')}>Explain Record</button>
                <button className={`ai-tab ${aiTab === 'report' ? 'active' : ''}`} onClick={() => setAiTab('report')}>Resolution Report</button>
              </div>

              <div className={`ai-pane ${aiTab === 'explain' ? 'active' : ''}`}>
                <div className="explain-row">
                  <input className="explain-input" value={explainId} onChange={e => setExplainId(e.target.value)} placeholder="Record ID, e.g. GW-20" />
                  <button className="btn btn-primary btn-sm" onClick={() => askAi()} disabled={aiLoading || !explainId}>Ask AI</button>
                </div>
                {aiLoading && aiTab === 'explain' ? (
                  <div className="ai-response loading"><Loader2 size={16} className="spin"/> Asking AI...</div>
                ) : aiResponse ? (
                  <div className="ai-response ai-markdown" dangerouslySetInnerHTML={{__html: aiResponse}} />
                ) : (
                  <div className="ai-response empty">Click an exception row below or enter a record ID to get a plain-English explanation.</div>
                )}
              </div>

              <div className={`ai-pane ${aiTab === 'report' ? 'active' : ''}`}>
                <div className="ai-report-meta">
                  <span>Full resolution instructions for all {results.exception_count} exceptions</span>
                  <button className="btn btn-sm btn-primary" onClick={generateReport} disabled={aiLoading}>
                    <FileText size={12}/> Generate Report
                  </button>
                </div>
                {aiLoading && aiTab === 'report' ? (
                  <div className="ai-response loading"><Loader2 size={16} className="spin"/> Generating resolution report...</div>
                ) : reportHTML ? (
                  <div className="ai-response ai-markdown" dangerouslySetInnerHTML={{__html: reportHTML}} />
                ) : (
                  <div className="ai-response empty">Click Generate Report to run the AI Finance Controller on all exceptions.</div>
                )}
              </div>
            </div>

          </section>
        )}
      </main>
    </>
  )
}
