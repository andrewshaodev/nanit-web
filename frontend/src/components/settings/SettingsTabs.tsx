import { useState, ReactNode } from 'react';

export interface SettingsTab {
  id: string;
  label: string;
  icon: string;
  content: ReactNode;
  disabled?: boolean;
}

interface SettingsTabsProps {
  tabs: SettingsTab[];
  defaultTab?: string;
}

export default function SettingsTabs({ tabs, defaultTab }: SettingsTabsProps) {
  const [activeTab, setActiveTab] = useState(defaultTab || tabs[0]?.id || '');

  const activeTabData = tabs.find(tab => tab.id === activeTab) || tabs[0];

  return (
    <div className="space-y-6">
      {/* Tab Navigation */}
      <div className="border-b border-ctp-surface0">
        <nav className="-mb-px flex space-x-8 overflow-x-auto">
          {tabs.map((tab) => (
            <button
              key={tab.id}
              onClick={() => !tab.disabled && setActiveTab(tab.id)}
              disabled={tab.disabled}
              className={`py-2 px-1 border-b-2 font-medium text-sm whitespace-nowrap flex items-center gap-2 ${
                activeTab === tab.id
                  ? 'border-ctp-blue text-ctp-blue-700 dark:text-ctp-blue'
                  : tab.disabled
                  ? 'border-transparent text-ctp-overlay1 cursor-not-allowed'
                  : 'border-transparent text-ctp-subtext1 hover:text-ctp-subtext1 hover:border-ctp-surface1'
              }`}
            >
              <span>{tab.icon}</span>
              {tab.label}
            </button>
          ))}
        </nav>
      </div>

      {/* Tab Content */}
      <div className="bg-ctp-base rounded-lg shadow-sm p-6">
        <div className="mb-4">
          <h3 className="text-lg font-medium text-ctp-text flex items-center gap-2">
            <span>{activeTabData.icon}</span>
            {activeTabData.label}
          </h3>
        </div>
        
        <div className="space-y-6">
          {activeTabData.content}
        </div>
      </div>
    </div>
  );
}