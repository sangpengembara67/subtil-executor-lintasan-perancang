// Dashboard JavaScript
class Dashboard {
    constructor() {
        this.currentView = 'dashboard';
        this.schedules = [];
        this.charts = {};
        this.init();
    }

    init() {
        this.setupEventListeners();
        this.setupNavigation();
        this.loadDashboardData();
        this.setupCharts();
        this.startDataRefresh();
    }

    setupEventListeners() {
        // Mobile menu toggle
        const mobileMenuBtn = document.getElementById('mobileMenuBtn');
        if (mobileMenuBtn) {
            mobileMenuBtn.addEventListener('click', () => {
                this.toggleSidebar();
            });
        }

        const closeSidebarBtn = document.getElementById('closeSidebarBtn');
        if (closeSidebarBtn) {
            closeSidebarBtn.addEventListener('click', () => {
                this.closeSidebar();
            });
        }

        const mobileOverlay = document.getElementById('mobileOverlay');
        if (mobileOverlay) {
            mobileOverlay.addEventListener('click', () => {
                this.closeSidebar();
            });
        }

        // Logout
        const logoutBtn = document.getElementById('logoutBtn');
        if (logoutBtn) {
            logoutBtn.addEventListener('click', () => {
                this.logout();
            });
        }

        // Schedule management
        const addScheduleBtn = document.getElementById('addScheduleBtn');
        if (addScheduleBtn) {
            addScheduleBtn.addEventListener('click', () => {
                this.showScheduleModal();
            });
        }

        const closeModal = document.getElementById('closeModal');
        if (closeModal) {
            closeModal.addEventListener('click', () => {
                this.hideScheduleModal();
            });
        }

        const cancelSchedule = document.getElementById('cancelSchedule');
         if (cancelSchedule) {
             cancelSchedule.addEventListener('click', () => {
                 this.hideScheduleModal();
             });
         }

       // Schedule form
        const scheduleForm = document.getElementById('scheduleForm');
        if (scheduleForm) {
            scheduleForm.addEventListener('submit', (e) => {
                e.preventDefault();
                this.addSchedule();
            });
        }

        // Settings forms
        const changePasswordForm = document.getElementById('changePasswordForm');
        if (changePasswordForm) {
            changePasswordForm.addEventListener('submit', (e) => {
                e.preventDefault();
                this.changePassword();
            });
        }

        const changeUsernameForm = document.getElementById('changeUsernameForm');
        if (changeUsernameForm) {
            changeUsernameForm.addEventListener('submit', (e) => {
                e.preventDefault();
                this.changeUsername();
            });
        }

        // AI Analysis controls
        const runChatAnalysis = document.getElementById('runChatAnalysis');
        if (runChatAnalysis) {
            runChatAnalysis.addEventListener('click', () => {
                this.runChatAnalysis();
            });
        }

        const runStockAnalysis = document.getElementById('runStockAnalysis');
        if (runStockAnalysis) {
            runStockAnalysis.addEventListener('click', () => {
                this.runStockAnalysis();
            });
        }

        // Close modal on outside click
        const scheduleModal = document.getElementById('scheduleModal');
        if (scheduleModal) {
            scheduleModal.addEventListener('click', (e) => {
                if (e.target.id === 'scheduleModal') {
                    this.hideScheduleModal();
                }
            });
        }
    }

    setupNavigation() {
        const navLinks = document.querySelectorAll('.nav-link');
        navLinks.forEach(link => {
            link.addEventListener('click', (e) => {
                e.preventDefault();
                const view = link.getAttribute('href').substring(1);
                this.switchView(view);
                
                // Update active nav
                navLinks.forEach(l => l.classList.remove('bg-white', 'bg-opacity-20'));
                link.classList.add('bg-white', 'bg-opacity-20');
                
                // Close sidebar on mobile
                if (window.innerWidth < 768) {
                    this.closeSidebar();
                }
            });
        });
    }

    switchView(viewName) {
        // Hide all views
        document.querySelectorAll('.view').forEach(view => {
            view.classList.add('hidden');
        });

        // Show selected view
        const targetView = document.getElementById(viewName + 'View');
        if (targetView) {
            targetView.classList.remove('hidden');
            targetView.classList.add('fade-in');
        }

        // Update page title
        const titles = {
            dashboard: 'Dashboard',
            schedules: 'Schedule Management',
            analytics: 'Analytics & Reports',
            settings: 'Settings'
        };
        document.getElementById('pageTitle').textContent = titles[viewName] || 'Dashboard';
        
        this.currentView = viewName;

        // Load view-specific data and setup charts
        switch (viewName) {
            case 'schedules':
                this.loadSchedules();
                break;
            case 'analytics':
                this.loadAnalytics();
                // Setup analytics charts when view becomes visible
                setTimeout(() => this.setupAnalyticsCharts(), 100);
                break;
        }
    }

    toggleSidebar() {
        const sidebar = document.getElementById('sidebar');
        const overlay = document.getElementById('mobileOverlay');
        
        sidebar.classList.toggle('sidebar-hidden');
        overlay.classList.toggle('hidden');
    }

    closeSidebar() {
        const sidebar = document.getElementById('sidebar');
        const overlay = document.getElementById('mobileOverlay');
        
        sidebar.classList.add('sidebar-hidden');
        overlay.classList.add('hidden');
    }

    async loadDashboardData() {
        try {
            const response = await fetch('/api/dashboard/stats');
            if (response.ok) {
                const data = await response.json();
                this.updateDashboardStats(data);
            }
        } catch (error) {
            console.error('Error loading dashboard data:', error);
        }
    }

    updateDashboardStats(data) {
        if (data.active_schedules !== undefined) {
            document.getElementById('activeSchedulesCount').textContent = data.active_schedules;
        }
        if (data.messages_analyzed !== undefined) {
            document.getElementById('messagesAnalyzed').textContent = data.messages_analyzed;
        }
        if (data.stock_mentions !== undefined) {
            document.getElementById('stockMentions').textContent = data.stock_mentions;
        }
    }

    async loadSchedules() {
        try {
            const response = await fetch('/api/schedules');
            if (response.ok) {
                this.schedules = await response.json();
                this.renderSchedulesTable();
                this.renderUpcomingSchedules();
            }
        } catch (error) {
            console.error('Error loading schedules:', error);
            this.showToast('Error loading schedules', 'error');
        }
    }

    renderSchedulesTable() {
        const tbody = document.getElementById('schedulesTable');
        tbody.innerHTML = '';

        this.schedules.forEach(schedule => {
            const row = document.createElement('tr');
            row.className = 'border-b border-gray-100 hover:bg-gray-50';
            
            const daysText = this.formatDays(schedule.days_of_week);
            const statusBadge = schedule.enabled ? 
                '<span class="bg-green-100 text-green-800 px-2 py-1 rounded-full text-xs font-medium">Active</span>' :
                '<span class="bg-red-100 text-red-800 px-2 py-1 rounded-full text-xs font-medium">Inactive</span>';

            row.innerHTML = `
                <td class="py-3 px-4">${schedule.chat_id}</td>
                <td class="py-3 px-4">
                    <span class="capitalize">${schedule.platform}</span>
                </td>
                <td class="py-3 px-4">${schedule.schedule_time}</td>
                <td class="py-3 px-4">${daysText}</td>
                <td class="py-3 px-4">${statusBadge}</td>
                <td class="py-3 px-4">
                    <div class="flex space-x-2">
                        <button onclick="dashboard.testSchedule(${schedule.id})" class="text-blue-600 hover:text-blue-800 text-sm">
                            <i class="fas fa-play"></i> Test
                        </button>
                        <button onclick="dashboard.toggleSchedule(${schedule.id})" class="text-yellow-600 hover:text-yellow-800 text-sm">
                            <i class="fas fa-${schedule.enabled ? 'pause' : 'play'}"></i> ${schedule.enabled ? 'Disable' : 'Enable'}
                        </button>
                        <button onclick="dashboard.deleteSchedule(${schedule.id})" class="text-red-600 hover:text-red-800 text-sm">
                            <i class="fas fa-trash"></i> Delete
                        </button>
                    </div>
                </td>
            `;
            
            tbody.appendChild(row);
        });
    }

    renderUpcomingSchedules() {
        const container = document.getElementById('upcomingSchedules');
        container.innerHTML = '';

        const activeSchedules = this.schedules.filter(s => s.enabled).slice(0, 5);
        
        if (activeSchedules.length === 0) {
            container.innerHTML = '<p class="text-gray-500 text-sm">No upcoming schedules</p>';
            return;
        }

        activeSchedules.forEach(schedule => {
            const div = document.createElement('div');
            div.className = 'flex items-center p-3 bg-gray-50 rounded-lg';
            
            const platformIcon = schedule.platform === 'whatsapp' ? 'fab fa-whatsapp text-green-500' : 'fab fa-telegram text-blue-500';
            
            div.innerHTML = `
                <i class="${platformIcon} mr-3"></i>
                <div>
                    <p class="text-sm font-medium">${schedule.chat_id}</p>
                    <p class="text-xs text-gray-500">${schedule.schedule_time} - ${this.formatDays(schedule.days_of_week)}</p>
                </div>
            `;
            
            container.appendChild(div);
        });
    }

    formatDays(daysString) {
        const dayNames = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'];
        const days = daysString.split(',').map(d => parseInt(d.trim()));
        return days.map(d => dayNames[d - 1]).join(', ');
    }

    showScheduleModal() {
        document.getElementById('scheduleModal').classList.remove('hidden');
    }

    hideScheduleModal() {
        document.getElementById('scheduleModal').classList.add('hidden');
        document.getElementById('scheduleForm').reset();
        // Reset default values
        document.getElementById('scheduleTime').value = '19:00';
        document.querySelectorAll('.day-checkbox').forEach((cb, index) => {
            cb.checked = index < 5; // Monday to Friday
        });
    }

    async addSchedule() {
        const chatId = document.getElementById('chatId').value;
        const platform = document.getElementById('platform').value;
        const scheduleTime = document.getElementById('scheduleTime').value;
        
        const selectedDays = Array.from(document.querySelectorAll('.day-checkbox:checked'))
            .map(cb => cb.value)
            .join(',');

        if (!selectedDays) {
            this.showToast('Please select at least one day', 'error');
            return;
        }

        try {
            const response = await fetch('/api/schedules', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({
                    chat_id: chatId,
                    platform: platform,
                    schedule_time: scheduleTime,
                    days_of_week: selectedDays,
                    enabled: true
                })
            });

            if (response.ok) {
                this.hideScheduleModal();
                this.loadSchedules();
                this.showToast('Schedule added successfully');
            } else {
                const error = await response.text();
                this.showToast('Error adding schedule: ' + error, 'error');
            }
        } catch (error) {
            console.error('Error adding schedule:', error);
            this.showToast('Error adding schedule', 'error');
        }
    }

    async testSchedule(scheduleId) {
        try {
            const response = await fetch(`/api/schedules/${scheduleId}/test`, {
                method: 'POST'
            });

            if (response.ok) {
                this.showToast('Test report sent successfully');
            } else {
                const error = await response.text();
                this.showToast('Error sending test: ' + error, 'error');
            }
        } catch (error) {
            console.error('Error testing schedule:', error);
            this.showToast('Error testing schedule', 'error');
        }
    }

    async toggleSchedule(scheduleId) {
        try {
            const response = await fetch(`/api/schedules/${scheduleId}/toggle`, {
                method: 'POST'
            });

            if (response.ok) {
                this.loadSchedules();
                this.showToast('Schedule updated successfully');
            } else {
                const error = await response.text();
                this.showToast('Error updating schedule: ' + error, 'error');
            }
        } catch (error) {
            console.error('Error toggling schedule:', error);
            this.showToast('Error updating schedule', 'error');
        }
    }

    async deleteSchedule(scheduleId) {
        if (!confirm('Are you sure you want to delete this schedule?')) {
            return;
        }

        try {
            const response = await fetch(`/api/schedules/${scheduleId}`, {
                method: 'DELETE'
            });

            if (response.ok) {
                this.loadSchedules();
                this.showToast('Schedule deleted successfully');
            } else {
                const error = await response.text();
                this.showToast('Error deleting schedule: ' + error, 'error');
            }
        } catch (error) {
            console.error('Error deleting schedule:', error);
            this.showToast('Error deleting schedule', 'error');
        }
    }

    async changePassword() {
        const currentPassword = document.getElementById('currentPassword').value;
        const newPassword = document.getElementById('newPassword').value;

        if (!currentPassword || !newPassword) {
            this.showToast('Please fill in all fields', 'error');
            return;
        }

        try {
            const response = await fetch('/api/user/change-password', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({
                    current_password: currentPassword,
                    new_password: newPassword
                })
            });

            if (response.ok) {
                document.getElementById('changePasswordForm').reset();
                this.showToast('Password changed successfully');
            } else {
                const error = await response.text();
                this.showToast('Error changing password: ' + error, 'error');
            }
        } catch (error) {
            console.error('Error changing password:', error);
            this.showToast('Error changing password', 'error');
        }
    }

    async changeUsername() {
        const newUsername = document.getElementById('newUsername').value;
        const confirmPassword = document.getElementById('confirmPassword').value;

        if (!newUsername || !confirmPassword) {
            this.showToast('Please fill in all fields', 'error');
            return;
        }

        try {
            const response = await fetch('/api/user/change-username', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({
                    new_username: newUsername,
                    password: confirmPassword
                })
            });

            if (response.ok) {
                document.getElementById('changeUsernameForm').reset();
                this.showToast('Username changed successfully');
                // Reload page to update displayed username
                setTimeout(() => location.reload(), 1500);
            } else {
                const error = await response.text();
                this.showToast('Error changing username: ' + error, 'error');
            }
        } catch (error) {
            console.error('Error changing username:', error);
            this.showToast('Error changing username', 'error');
        }
    }

    async loadAnalytics() {
        try {
            const response = await fetch('/api/analytics');
            if (response.ok) {
                const data = await response.json();
                this.updateCharts(data);
            }
        } catch (error) {
            console.error('Error loading analytics:', error);
        }
    }

    setupCharts() {
        // Check if Chart.js is loaded
        if (typeof Chart === 'undefined') {
            console.warn('Chart.js is not loaded yet, retrying in 100ms...');
            setTimeout(() => this.setupCharts(), 100);
            return;
        }

        // Message Analysis Chart
        const messageChartElement = document.getElementById('messageChart');
        if (messageChartElement) {
            const messageCtx = messageChartElement.getContext('2d');
            this.charts.messageChart = new Chart(messageCtx, {
                type: 'line',
                data: {
                    labels: [],
                    datasets: [{
                        label: 'Messages Analyzed',
                        data: [],
                        borderColor: 'rgb(59, 130, 246)',
                        backgroundColor: 'rgba(59, 130, 246, 0.1)',
                        tension: 0.4
                    }]
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    scales: {
                        y: {
                            beginAtZero: true
                        }
                    }
                }
            });
        }

        // Stock Mentions Chart
        const stockChartElement = document.getElementById('stockChart');
        if (stockChartElement) {
            const stockCtx = stockChartElement.getContext('2d');
            this.charts.stockChart = new Chart(stockCtx, {
                type: 'doughnut',
                data: {
                    labels: [],
                    datasets: [{
                        data: [],
                        backgroundColor: [
                            '#ef4444',
                            '#f97316',
                            '#eab308',
                            '#22c55e',
                            '#3b82f6',
                            '#8b5cf6',
                            '#ec4899'
                        ]
                    }]
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false
                }
            });
        }
        
        // Stock Mentions Analysis Chart - only setup if element exists and is visible
        const stockMentionsElement = document.getElementById('stockMentionsChart');
        if (stockMentionsElement && stockMentionsElement.offsetParent !== null) {
            const stockMentionsCtx = stockMentionsElement.getContext('2d');
            this.charts.stockMentions = new Chart(stockMentionsCtx, {
                type: 'bar',
                data: {
                    labels: [],
                    datasets: [{
                        label: 'Mentions',
                        data: [],
                        backgroundColor: 'rgba(59, 130, 246, 0.8)',
                        borderColor: 'rgb(59, 130, 246)',
                        borderWidth: 1
                    }]
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    scales: {
                        y: {
                            beginAtZero: true
                        }
                    }
                }
            });
        }
        
        // Stock Sentiment Chart - only setup if element exists and is visible
        const stockSentimentElement = document.getElementById('stockSentimentChart');
        if (stockSentimentElement && stockSentimentElement.offsetParent !== null) {
            const stockSentimentCtx = stockSentimentElement.getContext('2d');
            this.charts.stockSentiment = new Chart(stockSentimentCtx, {
                type: 'doughnut',
                data: {
                    labels: ['Positive', 'Neutral', 'Negative'],
                    datasets: [{
                        data: [0, 0, 0],
                        backgroundColor: [
                            '#22c55e',
                            '#6b7280',
                            '#ef4444'
                        ]
                    }]
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false
                }
            });
        }
    }

    setupAnalyticsCharts() {
        // Setup analytics charts when analytics view becomes visible
        const stockMentionsElement = document.getElementById('stockMentionsChart');
        if (stockMentionsElement && !this.charts.stockMentions) {
            const stockMentionsCtx = stockMentionsElement.getContext('2d');
            this.charts.stockMentions = new Chart(stockMentionsCtx, {
                type: 'bar',
                data: {
                    labels: [],
                    datasets: [{
                        label: 'Mentions',
                        data: [],
                        backgroundColor: 'rgba(59, 130, 246, 0.8)',
                        borderColor: 'rgb(59, 130, 246)',
                        borderWidth: 1
                    }]
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    scales: {
                        y: {
                            beginAtZero: true
                        }
                    }
                }
            });
        }
        
        const stockSentimentElement = document.getElementById('stockSentimentChart');
        if (stockSentimentElement && !this.charts.stockSentiment) {
            const stockSentimentCtx = stockSentimentElement.getContext('2d');
            this.charts.stockSentiment = new Chart(stockSentimentCtx, {
                type: 'doughnut',
                data: {
                    labels: ['Positive', 'Neutral', 'Negative'],
                    datasets: [{
                        data: [0, 0, 0],
                        backgroundColor: [
                            '#22c55e',
                            '#6b7280',
                            '#ef4444'
                        ]
                    }]
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false
                }
            });
        }
    }

    updateCharts(data) {
        // Update message chart
        if (data.message_trends) {
            this.charts.messageChart.data.labels = data.message_trends.labels;
            this.charts.messageChart.data.datasets[0].data = data.message_trends.data;
            this.charts.messageChart.update();
        }

        // Update stock chart
        if (data.stock_mentions) {
            this.charts.stockChart.data.labels = data.stock_mentions.labels;
            this.charts.stockChart.data.datasets[0].data = data.stock_mentions.data;
            this.charts.stockChart.update();
        }
    }

    startDataRefresh() {
        // Refresh dashboard data every 30 seconds
        setInterval(() => {
            if (this.currentView === 'dashboard') {
                this.loadDashboardData();
            }
        }, 30000);
    }

    showToast(message, type = 'success') {
        const toast = document.getElementById('toast');
        const toastMessage = document.getElementById('toastMessage');
        
        toastMessage.textContent = message;
        
        // Update toast styling based on type
        toast.className = `fixed top-4 right-4 px-6 py-3 rounded-lg shadow-lg z-50 ${
            type === 'error' ? 'bg-red-500' : 'bg-green-500'
        } text-white`;
        
        toast.classList.remove('hidden');
        
        // Auto hide after 3 seconds
        setTimeout(() => {
            toast.classList.add('hidden');
        }, 3000);
    }

    async runChatAnalysis() {
        const chatId = document.getElementById('chatAnalysisChatId').value;
        const days = document.getElementById('chatAnalysisDays').value;
        
        if (!chatId) {
            this.showToast('Please enter Chat ID', 'error');
            return;
        }
        
        const button = document.getElementById('runChatAnalysis');
        const originalText = button.textContent;
        button.textContent = 'Analyzing...';
        button.disabled = true;
        
        try {
            const response = await fetch(`/api/analyze/${chatId}?days=${days}`, {
                method: 'POST'
            });
            
            const result = await response.json();
            
            if (response.ok) {
                this.showToast('Chat analysis completed successfully');
                this.loadAnalytics(); // Refresh analytics data
            } else {
                this.showToast(result.error || 'Error running chat analysis', 'error');
            }
        } catch (error) {
            console.error('Error running chat analysis:', error);
            this.showToast('Error running chat analysis', 'error');
        } finally {
            button.textContent = originalText;
            button.disabled = false;
        }
    }
    
    async runStockAnalysis() {
        const chatId = document.getElementById('stockAnalysisChatId').value;
        const days = document.getElementById('stockAnalysisDays').value;
        
        if (!chatId) {
            this.showToast('Please enter Chat ID', 'error');
            return;
        }
        
        const button = document.getElementById('runStockAnalysis');
        const originalText = button.textContent;
        button.textContent = 'Analyzing...';
        button.disabled = true;
        
        try {
            const response = await fetch(`/api/stocks/mentions/${chatId}?days=${days}`);
            const result = await response.json();
            
            if (response.ok) {
                this.updateStockAnalysis(result);
                this.showToast('Stock analysis completed successfully');
            } else {
                this.showToast(result.error || 'Error running stock analysis', 'error');
            }
        } catch (error) {
            console.error('Error running stock analysis:', error);
            this.showToast('Error running stock analysis', 'error');
        } finally {
            button.textContent = originalText;
            button.disabled = false;
        }
    }
    
    updateStockAnalysis(data) {
        // Update stock mentions chart
        if (this.charts.stockMentions && data.stock_mentions) {
            const labels = data.stock_mentions.map(item => item.stock_code);
            const counts = data.stock_mentions.map(item => item.count);
            
            this.charts.stockMentions.data.labels = labels;
            this.charts.stockMentions.data.datasets[0].data = counts;
            this.charts.stockMentions.update();
        }
        
        // Update stock sentiment chart
        if (this.charts.stockSentiment && data.sentiment_distribution) {
            const sentiments = ['positive', 'neutral', 'negative'];
            const sentimentData = sentiments.map(sentiment => 
                data.sentiment_distribution[sentiment] || 0
            );
            
            this.charts.stockSentiment.data.datasets[0].data = sentimentData;
            this.charts.stockSentiment.update();
        }
        
        // Update recent mentions list
        this.updateRecentMentions(data.recent_mentions || []);
    }
    
    updateRecentMentions(mentions) {
        const container = document.getElementById('recentMentionsList');
        if (!container) return;
        
        container.innerHTML = '';
        
        mentions.slice(0, 10).forEach(mention => {
            const item = document.createElement('div');
            item.className = 'flex justify-between items-center p-3 bg-gray-50 rounded-lg';
            
            const sentimentColor = {
                'positive': 'text-green-600',
                'negative': 'text-red-600',
                'neutral': 'text-gray-600'
            }[mention.sentiment] || 'text-gray-600';
            
            item.innerHTML = `
                <div>
                    <span class="font-semibold text-blue-600">${mention.stock_code}</span>
                    <p class="text-sm text-gray-600 truncate" style="max-width: 200px;">${mention.context}</p>
                </div>
                <div class="text-right">
                    <span class="${sentimentColor} text-sm font-medium">${mention.sentiment}</span>
                    <p class="text-xs text-gray-500">${new Date(mention.timestamp).toLocaleDateString()}</p>
                </div>
            `;
            
            container.appendChild(item);
        });
    }

    async logout() {
        try {
            const response = await fetch('/api/logout', {
                method: 'POST'
            });
            
            if (response.ok) {
                window.location.href = '/login';
            } else {
                this.showToast('Error logging out', 'error');
            }
        } catch (error) {
            console.error('Error logging out:', error);
            this.showToast('Error logging out', 'error');
        }
    }
}

// Initialize dashboard when DOM is loaded
let dashboard;
document.addEventListener('DOMContentLoaded', () => {
    // Wait for Chart.js to load before initializing dashboard
    if (typeof Chart !== 'undefined') {
        dashboard = new Dashboard();
    } else {
        // Poll for Chart.js availability
        const checkChart = setInterval(() => {
            if (typeof Chart !== 'undefined') {
                clearInterval(checkChart);
                dashboard = new Dashboard();
            }
        }, 50);
    }
});

// Handle window resize for responsive sidebar
window.addEventListener('resize', () => {
    if (window.innerWidth >= 768) {
        const sidebar = document.getElementById('sidebar');
        const overlay = document.getElementById('mobileOverlay');
        sidebar.classList.remove('sidebar-hidden');
        overlay.classList.add('hidden');
    }
});