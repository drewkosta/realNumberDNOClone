import { useState } from 'react';
import { roiApi } from '../api';
import type { ROICalculation } from '../types';
import { Calculator, TrendingUp, Shield, DollarSign } from 'lucide-react';

export default function ROIPage() {
  const [volume, setVolume] = useState(50000);
  const [result, setResult] = useState<ROICalculation | null>(null);
  const [loading, setLoading] = useState(false);

  const handleCalculate = async () => {
    setLoading(true);
    try {
      const data = await roiApi.calculate(volume);
      setResult(data);
    } finally {
      setLoading(false);
    }
  };

  const riskColors: Record<string, string> = {
    low: 'bg-green-100 text-green-700',
    medium: 'bg-amber-100 text-amber-700',
    high: 'bg-red-100 text-red-700',
  };

  return (
    <div className="max-w-3xl mx-auto">
      <div className="mb-8 animate-fade-up text-center">
        <h1 className="text-2xl font-bold text-gray-900">ROI Calculator</h1>
        <p className="text-gray-500 mt-1">Estimate the value of DNO enforcement based on your traffic volume</p>
      </div>

      <div className="card-hover bg-white rounded-xl p-8 shadow-sm border border-gray-100 mb-8 animate-fade-up stagger-1">
        <div className="text-center mb-6">
          <label className="block text-sm font-medium text-gray-700 mb-3">Daily Inbound Call Volume</label>
          <input
            type="range"
            min={1000}
            max={1000000}
            step={1000}
            value={volume}
            onChange={(e) => setVolume(Number(e.target.value))}
            className="w-full h-2 bg-gray-200 rounded-lg appearance-none cursor-pointer accent-blue-600"
          />
          <div className="flex justify-between text-xs text-gray-400 mt-1">
            <span>1K</span><span>250K</span><span>500K</span><span>750K</span><span>1M</span>
          </div>
          <p className="text-4xl font-bold text-gray-900 mt-4 animate-count-up">{volume.toLocaleString()}</p>
          <p className="text-sm text-gray-500">calls per day</p>
        </div>
        <button
          onClick={() => void handleCalculate()}
          disabled={loading}
          className="w-full flex items-center justify-center gap-2 px-6 py-3 bg-blue-600 text-white rounded-lg font-medium hover:bg-blue-700 hover:shadow-lg hover:shadow-blue-600/25 disabled:opacity-50 transition-all duration-200"
        >
          <Calculator className="w-5 h-5" />
          {loading ? 'Calculating...' : 'Calculate ROI'}
        </button>
      </div>

      {result && (
        <div className="animate-fade-up">
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-6">
            <div className="card-hover bg-white rounded-xl p-6 shadow-sm border border-gray-100 text-center animate-fade-up stagger-1">
              <Shield className="w-8 h-8 text-blue-500 mx-auto mb-2" />
              <p className="text-3xl font-bold text-gray-900 animate-count-up">{result.estDailyBlocked.toLocaleString()}</p>
              <p className="text-sm text-gray-500">Blocked per Day</p>
            </div>
            <div className="card-hover bg-white rounded-xl p-6 shadow-sm border border-gray-100 text-center animate-fade-up stagger-2">
              <TrendingUp className="w-8 h-8 text-green-500 mx-auto mb-2" />
              <p className="text-3xl font-bold text-gray-900 animate-count-up">{result.estAnnualBlocked.toLocaleString()}</p>
              <p className="text-sm text-gray-500">Blocked per Year</p>
            </div>
            <div className="card-hover bg-green-50 rounded-xl p-6 shadow-sm border border-green-200 text-center animate-fade-up stagger-3">
              <DollarSign className="w-8 h-8 text-green-600 mx-auto mb-2" />
              <p className="text-3xl font-bold text-green-700 animate-count-up">${result.estAnnualSavings.toLocaleString()}</p>
              <p className="text-sm text-green-600">Est. Annual Savings</p>
            </div>
          </div>

          <div className="card-hover bg-white rounded-xl p-6 shadow-sm border border-gray-100 animate-fade-up stagger-4">
            <h3 className="text-lg font-semibold text-gray-900 mb-4">Calculation Breakdown</h3>
            <div className="space-y-3 text-sm">
              <div className="flex justify-between py-2 border-b border-gray-50">
                <span className="text-gray-500">Industry Average DNO Hit Rate</span>
                <span className="font-medium">{result.estHitRate}%</span>
              </div>
              <div className="flex justify-between py-2 border-b border-gray-50">
                <span className="text-gray-500">Estimated Monthly Blocked</span>
                <span className="font-medium">{result.estMonthlyBlocked.toLocaleString()}</span>
              </div>
              <div className="flex justify-between py-2 border-b border-gray-50">
                <span className="text-gray-500">Avg Cost per Robocall Complaint</span>
                <span className="font-medium">${result.avgComplaintCost.toFixed(2)}</span>
              </div>
              <div className="flex justify-between py-2">
                <span className="text-gray-500">Compliance Risk Level (without DNO)</span>
                <span className={`px-2 py-0.5 rounded-full text-xs font-medium ${riskColors[result.complianceRiskLevel] ?? 'bg-gray-100 text-gray-600'}`}>
                  {result.complianceRiskLevel.toUpperCase()}
                </span>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
