import { useState } from 'react'
import { marked } from 'marked'
import {
  Upload, Sparkles, CheckCircle2, XCircle, Activity, Play, FileText, ChevronDown, ChevronUp, Loader2
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

  const getReasonBadge = (reason) => {
    const classes = {
      'AMOUNT_MISMATCH': 'bg-[#FAEEE9] text-semantic-error border-[#E8C4BB]',
      'DATE_DRIFT': 'bg-[#FDF6EC] text-semantic-warning border-[#E8D4A0]',
      'DUPLICATE_REF': 'bg-[#EEF2F9] text-[#4A6CA8] border-[#B8C8E8]',
      'NO_COUNTERPART': 'bg-[#F0EEF6] text-[#6A5A8A] border-[#C8BCE8]',
      'MALFORMED_INPUT': 'bg-[#F0F0F0] text-[#555] border-[#CCC]',
    }
    const color = classes[reason] || 'bg-gray-100 text-gray-700 border-gray-300'
    return `inline-block font-mono text-[10px] font-medium px-[7px] py-[2px] rounded-[3px] uppercase border ${color}`
  }

  const getPassBadge = (pass) => {
    const isExact = pass === 'exact'
    const color = isExact ? 'bg-[#F0F6F1] text-semantic-success border-[#B4D4BA]' : 'bg-[#FDF6EC] text-semantic-warning border-[#E8D4A0]'
    return `inline-block font-mono text-[10px] px-[7px] py-[2px] rounded-[3px] uppercase border ${color}`
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

  // Reusable button classes
  const btnClass = "inline-flex items-center gap-[7px] px-5 py-2.5 border-[1.5px] border-paper-ink rounded-md bg-paper-surface text-paper-ink font-sans text-[13px] font-medium cursor-pointer transition-all hover:bg-paper-ink hover:text-paper-bg hover:shadow-none disabled:opacity-45 disabled:cursor-not-allowed whitespace-nowrap"
  const btnPrimary = "bg-paper-ink text-paper-bg shadow-hard hover:bg-[#333] hover:shadow-[1px_1px_0_#1A1A1A]"
  
  return (
    <>
      <header className="flex items-center justify-between px-8 h-14 bg-paper-surface border-b border-paper-ink sticky top-0 z-[100]">
        <div className="flex items-center gap-2.5 font-semibold text-[15px] tracking-[-0.3px]">
          <div className="w-7 h-7 bg-paper-ink rounded-md flex items-center justify-center text-paper-bg font-mono text-xs font-medium shrink-0">R</div>
          reconcile
        </div>
        <div className="text-xs text-paper-muted tracking-[0.04em] uppercase">AI Finance Controller</div>
        <div className="flex items-center gap-1.5 text-xs text-paper-muted px-2.5 py-1 border border-paper-rule rounded-full font-mono">
          <div className={`w-[7px] h-[7px] rounded-full ${status === 'running' ? 'bg-semantic-warning animate-[pulse_1s_infinite]' : 'bg-semantic-success'}`}></div>
          <span>{statusText}</span>
        </div>
      </header>

      <main className="max-w-[1100px] mx-auto px-6 pt-10 pb-20">
        {!results ? (
          <section id="uploadSection">
            <div className="text-center mb-10">
              <h1 className="text-[28px] font-semibold tracking-[-0.5px] mb-2">Multi-source Financial Reconciliation</h1>
              <p className="text-paper-muted text-sm">Upload your source CSV files, configure tolerances, and run the engine.</p>
            </div>

            <button className={`${btnClass} w-full justify-center mb-3 text-[13px] text-paper-muted border-paper-rule hover:text-paper-ink`} onClick={runSample} disabled={loading}>
              <Play size={14} /> Use Built-in Sample Data (74 records)
            </button>

            <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-6">
              {['gateway', 'bank', 'ledger'].map(source => (
                <div 
                  key={source}
                  className={`bg-paper-surface border-[1.5px] border-dashed rounded-[10px] py-8 px-5 text-center cursor-pointer transition-all relative ${files[source] ? 'border-solid border-semantic-success bg-[#F0F6F1]' : 'border-[#A09888] hover:border-paper-ink hover:bg-[#F0EDE3]'} [&.drag-over]:border-paper-ink [&.drag-over]:bg-[#F0EDE3]`}
                  onDragOver={e => { e.preventDefault(); e.currentTarget.classList.add('drag-over') }}
                  onDragLeave={e => e.currentTarget.classList.remove('drag-over')}
                  onDrop={e => handleDrop(e, source)}
                >
                  <input type="file" accept=".csv" className="absolute inset-0 opacity-0 cursor-pointer w-full h-full" onChange={e => handleFile(e, source)} />
                  <div className={`w-9 h-9 mx-auto mb-3 border-[1.5px] border-paper-rule rounded-lg flex items-center justify-center ${files[source] ? 'border-semantic-success text-semantic-success' : 'text-paper-muted'}`}>
                    {files[source] ? <CheckCircle2 size={16} /> : <Upload size={16} />}
                  </div>
                  <div className="text-[11px] font-semibold tracking-[0.08em] uppercase text-paper-muted mb-1">{source} CSV</div>
                  <div className="text-xs text-[#A09888]">{files[source] ? '✓ ready' : `${source}_file.csv`}</div>
                  {files[source] && <div className="font-mono text-[11px] text-semantic-success mt-1 break-all">{files[source].name}</div>}
                </div>
              ))}
            </div>

            <div className="bg-paper-surface border border-paper-ink rounded-[10px] shadow-hard-sm px-6 py-5 mb-6">
              <div className="text-[11px] font-semibold tracking-[0.08em] uppercase text-paper-muted mb-4">Matching Configuration</div>
              <div className="grid grid-cols-1 md:grid-cols-3 gap-5">
                {[
                  { id: 'amtTol', label: 'Amount Tolerance', min: 0, max: 10, step: 0.5, suffix: '%' },
                  { id: 'dateWin', label: 'Date Window (days)', min: 0, max: 14, step: 1, suffix: 'd' },
                  { id: 'minConf', label: 'Min Confidence', min: 0.5, max: 1, step: 0.05, suffix: '' }
                ].map(c => (
                  <div key={c.id}>
                    <label className="block text-[11px] text-paper-muted tracking-[0.04em] uppercase mb-2">{c.label}</label>
                    <div className="flex items-center gap-2.5">
                      <input type="range" min={c.min} max={c.max} step={c.step} value={config[c.id]} onChange={e => setConfig({...config, [c.id]: e.target.value})} 
                        className="flex-1 h-[3px] bg-paper-rule rounded-sm outline-none appearance-none [&::-webkit-slider-thumb]:appearance-none [&::-webkit-slider-thumb]:w-3.5 [&::-webkit-slider-thumb]:h-3.5 [&::-webkit-slider-thumb]:bg-paper-ink [&::-webkit-slider-thumb]:rounded-full [&::-webkit-slider-thumb]:cursor-pointer" />
                      <span className="font-mono text-[13px] font-medium min-w-[42px] text-right">{config[c.id]}{c.suffix}</span>
                    </div>
                  </div>
                ))}
              </div>
            </div>

            <button className={`${btnClass} ${btnPrimary} w-full justify-center p-3.5 text-sm`} onClick={runReconcile} disabled={!isReadyToRun || loading}>
              {loading ? <Loader2 size={16} className="spin" /> : <Play size={16} />}
              Run Reconciliation
            </button>
          </section>
        ) : (
          <section id="resultsSection">
            <div className="flex items-center justify-between mb-7">
              <div className="text-[13px] text-paper-muted tracking-[0.04em] uppercase font-semibold">Reconciliation Report</div>
              <div className="flex items-center gap-3">
                <span className="font-mono text-[11px] text-paper-muted bg-paper-surface border border-paper-rule px-2 py-[3px] rounded-md">{results.elapsed_ms.toFixed(2)} ms</span>
                <button className={`${btnClass} px-3 py-1.5 text-xs`} onClick={reset}>
                  <Upload size={13} /> New Run
                </button>
              </div>
            </div>

            <div className="grid grid-cols-3 md:grid-cols-6 gap-3 mb-5">
              {[
                { label: 'Total Records', val: results.total_records, color: '' },
                { label: 'Match Rate', val: `${results.match_rate_pct.toFixed(2)}%`, color: 'text-semantic-success' },
                { label: 'Exact Matches', val: results.exact_matches, color: '' },
                { label: 'Fuzzy Matches', val: results.fuzzy_matches, color: 'text-semantic-warning' },
                { label: 'Exceptions', val: results.exception_count, color: 'text-semantic-error' },
                { label: 'FP Risk', val: results.fp_risk, color: 'text-semantic-warning', sub: `${results.fp_rate_pct.toFixed(2)}% of total` }
              ].map((s, i) => (
                <div key={i} className="bg-paper-surface border border-paper-ink rounded-[10px] shadow-hard-sm px-3.5 py-4">
                  <div className="text-[10px] font-semibold tracking-[0.07em] uppercase text-paper-muted mb-2">{s.label}</div>
                  <div className={`font-mono text-[22px] font-medium leading-[1.1] ${s.color}`}>{s.val}</div>
                  {s.sub && <div className="font-mono text-[10px] text-paper-muted mt-1">{s.sub}</div>}
                </div>
              ))}
            </div>

            <div className={`flex items-center gap-3 px-4 py-3 rounded-md border mb-6 text-[13px] ${results.invariant_valid ? 'border-semantic-success bg-[#F0F6F1] text-semantic-success' : 'border-semantic-error bg-[#FAF0ED] text-semantic-error'}`}>
              {results.invariant_valid ? <CheckCircle2 size={16} className="shrink-0"/> : <XCircle size={16} className="shrink-0"/>}
              <span className="font-mono text-xs">
                {results.invariant_valid 
                  ? `Integrity Verified — ${results.exact_matches + results.fuzzy_matches} matched + ${results.exception_count} exceptions = ${results.total_records} total`
                  : 'Integrity Violated — counts do not add up'}
              </span>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-6">
              {[
                { title: 'Exception Breakdown', map: results.exception_breakdown, max: results.exception_count, color: 'bg-semantic-error' },
                { title: 'Records by Source', map: results.records_by_source, max: results.total_records, color: 'bg-paper-ink' }
              ].map((c, i) => (
                <div key={i} className="bg-paper-surface border border-paper-ink rounded-[10px] shadow-hard-sm p-5">
                  <div className="text-[11px] font-semibold tracking-[0.07em] uppercase text-paper-muted mb-4">{c.title}</div>
                  {Object.entries(c.map).sort((a,b)=>b[1]-a[1]).map(([k,v]) => (
                    <div className="flex items-center gap-2.5 mb-3 last:mb-0" key={k}>
                      <div className="font-mono text-[11px] w-[140px] shrink-0 text-paper-ink">{k}</div>
                      <div className="flex-1 h-1.5 bg-paper-rule rounded-sm overflow-hidden">
                        <div className={`h-full rounded-sm transition-all duration-500 ${c.color}`} style={{width: `${(v / c.max) * 100}%`}}></div>
                      </div>
                      <div className="font-mono text-[11px] text-paper-muted w-[30px] text-right shrink-0">{v}</div>
                    </div>
                  ))}
                </div>
              ))}
            </div>

            {results.alerts && results.alerts.length > 0 && (
              <div className="mb-6">
                <div className="text-[11px] font-semibold tracking-[0.07em] uppercase text-paper-muted mb-3">
                  <Activity size={13} style={{display:'inline', verticalAlign:'-2px', marginRight:4}}/>
                  Systemic Alerts
                </div>
                {results.alerts.map((a, i) => (
                  <div className="bg-[#FDF6EC] border border-semantic-warning rounded-[10px] px-[18px] py-4 mb-2.5 last:mb-0" key={i}>
                    <div className="flex items-center gap-2.5 mb-1.5">
                      <span className="font-mono text-[10px] font-medium bg-semantic-warning text-white px-[7px] py-[2px] rounded-[3px] uppercase">{a.reason_code}</span>
                      <span className="font-mono text-[10px] font-medium bg-paper-muted text-white px-[7px] py-[2px] rounded-[3px] uppercase">{a.source}</span>
                      <span className="font-mono text-[11px] text-paper-muted">count: {a.count} (threshold: {a.threshold})</span>
                    </div>
                    <div className="text-[13px] text-[#7A5A1A] leading-[1.5]">{a.message}</div>
                  </div>
                ))}
              </div>
            )}

            <div className="bg-paper-surface border border-paper-ink rounded-[10px] shadow-hard-sm mb-4" style={{overflow:'hidden'}}>
              <div className="pt-4 px-5">
                <div className="text-[11px] font-semibold tracking-[0.07em] uppercase text-paper-muted mb-3">Exceptions</div>
                <div className="flex items-center gap-2 mb-3.5 flex-wrap">
                  {['ALL', 'AMOUNT_MISMATCH', 'DATE_DRIFT', 'DUPLICATE_REF', 'NO_COUNTERPART', 'MALFORMED_INPUT'].map(f => (
                    <button key={f} className={`px-3 py-1.5 border border-paper-rule rounded-full text-[11px] font-medium cursor-pointer transition-all tracking-[0.03em] font-mono hover:border-paper-ink hover:text-paper-ink ${activeFilter === f ? 'bg-paper-ink text-paper-bg border-paper-ink' : 'bg-paper-surface text-paper-muted'}`} onClick={() => setActiveFilter(f)}>
                      {f.replace(/_/g, ' ')}
                    </button>
                  ))}
                  <input type="text" className="ml-auto px-3 py-1.5 border border-paper-rule rounded-md bg-paper-surface font-mono text-xs text-paper-ink outline-none w-[160px] focus:border-paper-ink placeholder:text-[#A09888]" placeholder="Search records..." value={searchQuery} onChange={e => setSearchQuery(e.target.value)} />
                </div>
              </div>
              <div className="overflow-x-auto">
                <table className="w-full border-collapse text-[13px]">
                  <thead>
                    <tr>
                      {['Record ID', 'Source', 'Ref ID', 'Amount', 'Reason', 'Detail', ''].map(h => (
                        <th key={h} className="text-left text-[10px] font-semibold tracking-[0.07em] uppercase text-paper-muted px-3 py-2 border-b border-paper-ink bg-paper-surface sticky top-0 z-10">{h}</th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {filteredExceptions.map(e => (
                      <tr key={e.record_id} className={`cursor-pointer hover:bg-[#F0EDE3] ${explainId === e.record_id ? 'bg-[#EEEBE1]' : ''}`} onClick={() => askAi(e.record_id)}>
                        <td className="px-3 py-2.5 border-b border-paper-rule align-middle"><span className="font-mono text-xs">{e.record_id}</span></td>
                        <td className="px-3 py-2.5 border-b border-paper-rule align-middle"><span className="font-mono text-xs text-paper-muted">{e.source}</span></td>
                        <td className="px-3 py-2.5 border-b border-paper-rule align-middle"><span className="font-mono text-xs">{e.ref_id}</span></td>
                        <td className="px-3 py-2.5 border-b border-paper-rule align-middle"><span className="font-mono text-xs">{e.amount.toFixed(2)} <span className="text-paper-muted">{e.currency}</span></span></td>
                        <td className="px-3 py-2.5 border-b border-paper-rule align-middle"><span className={getReasonBadge(e.reason_code)}>{e.reason_code.replace(/_/g,' ')}</span></td>
                        <td className="px-3 py-2.5 border-b border-paper-rule align-middle" style={{maxWidth:220, whiteSpace:'nowrap', overflow:'hidden', textOverflow:'ellipsis'}}>
                          <span className="text-xs text-paper-muted">{e.detail}</span>
                        </td>
                        <td className="px-3 py-2.5 border-b border-paper-rule align-middle">
                          <button className={`${btnClass} px-2 py-1 text-[11px]`} onClick={ev => {ev.stopPropagation(); askAi(e.record_id)}}>Ask AI</button>
                        </td>
                      </tr>
                    ))}
                    {filteredExceptions.length === 0 && (
                      <tr><td colSpan="7"><div className="text-center py-10 px-5 text-paper-muted text-[13px]">No exceptions match the current filter.</div></td></tr>
                    )}
                  </tbody>
                </table>
              </div>
            </div>

            <div className="mb-4">
              <button className="w-full flex items-center justify-between px-5 py-3 bg-paper-surface border border-paper-ink rounded-[10px] cursor-pointer text-xs font-semibold tracking-[0.05em] uppercase text-paper-muted shadow-hard-sm hover:bg-[#F0EDE3]" onClick={() => setMatchesOpen(!matchesOpen)} style={matchesOpen ? {borderBottomLeftRadius: 0, borderBottomRightRadius: 0, marginBottom: 0} : {}}>
                <span>Matched Records</span>
                {matchesOpen ? <ChevronUp size={14}/> : <ChevronDown size={14}/>}
              </button>
              {matchesOpen && (
                <div className="border border-paper-ink border-t-0 rounded-b-[10px] overflow-hidden bg-paper-surface">
                  <div className="overflow-x-auto">
                    <table className="w-full border-collapse text-[13px]">
                      <thead>
                        <tr>
                          {['Match ID', 'Ref ID', 'Pass', 'Sources', 'Records', 'Confidence'].map(h => (
                            <th key={h} className="text-left text-[10px] font-semibold tracking-[0.07em] uppercase text-paper-muted px-3 py-2 border-b border-paper-ink bg-paper-surface">{h}</th>
                          ))}
                        </tr>
                      </thead>
                      <tbody>
                        {results.matches.map(m => (
                          <tr key={m.match_id}>
                            <td className="px-3 py-2.5 border-b border-paper-rule align-middle"><span className="font-mono text-[11px]">{m.match_id}</span></td>
                            <td className="px-3 py-2.5 border-b border-paper-rule align-middle"><span className="font-mono text-xs">{m.ref_id}</span></td>
                            <td className="px-3 py-2.5 border-b border-paper-rule align-middle"><span className={getPassBadge(m.pass)}>{m.pass}</span></td>
                            <td className="px-3 py-2.5 border-b border-paper-rule align-middle"><span className="font-mono text-[11px] text-paper-muted">{m.sources.join(' · ')}</span></td>
                            <td className="px-3 py-2.5 border-b border-paper-rule align-middle"><span className="font-mono text-xs text-paper-muted">{m.record_count}</span></td>
                            <td className="px-3 py-2.5 border-b border-paper-rule align-middle">
                              <div className="flex items-center gap-1.5">
                                <div className="w-12 h-1 bg-paper-rule rounded-sm">
                                  <div className="h-full rounded-sm bg-semantic-success" style={{width: `${m.confidence * 100}%`}}></div>
                                </div>
                                <span className="font-mono text-[11px] text-paper-muted">{m.confidence.toFixed(3)}</span>
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

            <div className="bg-paper-surface border border-paper-ink rounded-[10px] shadow-hard overflow-hidden mt-6">
              <div className="flex items-center gap-2.5 px-5 py-4 border-b border-paper-rule bg-paper-ink text-paper-bg">
                <Sparkles size={15}/>
                <span className="text-xs font-semibold tracking-[0.05em] uppercase">AI Finance Controller</span>
                <span className="ml-auto font-mono text-[10px] opacity-60">Powered by Gemini</span>
              </div>
              <div className="flex border-b border-paper-rule">
                <button className={`px-5 py-[11px] text-xs cursor-pointer border-b-2 transition-all font-sans ${aiTab === 'explain' ? 'text-paper-ink border-paper-ink font-semibold' : 'font-medium text-paper-muted border-transparent hover:text-paper-ink'}`} onClick={() => setAiTab('explain')}>Explain Record</button>
                <button className={`px-5 py-[11px] text-xs cursor-pointer border-b-2 transition-all font-sans ${aiTab === 'report' ? 'text-paper-ink border-paper-ink font-semibold' : 'font-medium text-paper-muted border-transparent hover:text-paper-ink'}`} onClick={() => setAiTab('report')}>Resolution Report</button>
              </div>

              <div className={`p-5 ${aiTab === 'explain' ? 'block' : 'hidden'}`}>
                <div className="flex gap-2.5 mb-4">
                  <input className="flex-1 px-3.5 py-[9px] border border-paper-rule rounded-md bg-paper-bg font-mono text-[13px] text-paper-ink outline-none focus:border-paper-ink" value={explainId} onChange={e => setExplainId(e.target.value)} placeholder="Record ID, e.g. GW-20" />
                  <button className={`${btnClass} ${btnPrimary} px-3 py-1.5 text-xs`} onClick={() => askAi()} disabled={aiLoading || !explainId}>Ask AI</button>
                </div>
                {aiLoading && aiTab === 'explain' ? (
                  <div className="bg-paper-bg border border-paper-rule rounded-md p-4 min-h-[80px] text-[13px] leading-[1.65] text-paper-muted flex items-center gap-2.5"><Loader2 size={16} className="spin"/> Asking AI...</div>
                ) : aiResponse ? (
                  <div className="bg-paper-bg border border-paper-rule rounded-md p-4 min-h-[80px] text-[13px] leading-[1.65] text-paper-ink ai-markdown" dangerouslySetInnerHTML={{__html: aiResponse}} />
                ) : (
                  <div className="bg-paper-bg border border-paper-rule rounded-md p-4 min-h-[80px] text-[13px] leading-[1.65] text-paper-muted italic flex items-center justify-center">Click an exception row below or enter a record ID to get a plain-English explanation.</div>
                )}
              </div>

              <div className={`p-5 ${aiTab === 'report' ? 'block' : 'hidden'}`}>
                <div className="flex items-center justify-between mb-3">
                  <span className="text-[11px] text-paper-muted font-mono">Full resolution instructions for all {results?.exception_count || 0} exceptions</span>
                  <button className={`${btnClass} ${btnPrimary} px-3 py-1.5 text-xs`} onClick={generateReport} disabled={aiLoading}>
                    <FileText size={12}/> Generate Report
                  </button>
                </div>
                {aiLoading && aiTab === 'report' ? (
                  <div className="bg-paper-bg border border-paper-rule rounded-md p-4 min-h-[80px] text-[13px] leading-[1.65] text-paper-muted flex items-center gap-2.5"><Loader2 size={16} className="spin"/> Generating resolution report...</div>
                ) : reportHTML ? (
                  <div className="bg-paper-bg border border-paper-rule rounded-md p-4 min-h-[80px] text-[13px] leading-[1.65] text-paper-ink ai-markdown" dangerouslySetInnerHTML={{__html: reportHTML}} />
                ) : (
                  <div className="bg-paper-bg border border-paper-rule rounded-md p-4 min-h-[80px] text-[13px] leading-[1.65] text-paper-muted italic flex items-center justify-center">Click Generate Report to run the AI Finance Controller on all exceptions.</div>
                )}
              </div>
            </div>

          </section>
        )}
      </main>
    </>
  )
}
