/**
 * Vigil Dashboard - Utility Functions
 */

const Utils = {
    formatSize(bytes) {
        if (!bytes) return 'N/A';
        const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
        const i = Math.floor(Math.log(bytes) / Math.log(1024));
        return `${(bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0)} ${sizes[i]}`;
    },

    formatAge(hours) {
        if (!hours) return 'N/A';
        const years = hours / 8760;
        if (years >= 1) return `${years.toFixed(1)}y`;
        const months = hours / 730;
        if (months >= 1) return `${months.toFixed(1)}mo`;
        const days = hours / 24;
        if (days >= 1) return `${days.toFixed(0)}d`;
        return `${hours}h`;
    },

    formatTime(timestamp) {
        return new Date(timestamp).toLocaleTimeString([], { 
            hour: '2-digit', 
            minute: '2-digit' 
        });
    },

    getHealthStatus(drive) {
        if (!drive.smart_status?.passed) return 'critical';
        
        const attrs = drive.ata_smart_attributes?.table || [];
        const criticalIds = [5, 187, 197, 198];
        
        for (const attr of attrs) {
            if (criticalIds.includes(attr.id) && attr.raw?.value > 0) {
                return 'warning';
            }
        }
        
        return 'healthy';
    },

    getDriveName(drive) {
        if (drive._alias) return drive._alias;
        if (drive.model_name) return drive.model_name;
        if (drive.model_number) return drive.model_number;
        if (drive.scsi_model_name) return drive.scsi_model_name;
        if (drive.model_family) return drive.model_family;
        if (drive.device?.model) return drive.device.model;
        
        if (drive.device?.name) {
            const name = drive.device.name.replace('/dev/', '');
            if (drive.serial_number) {
                return `${name} (${drive.serial_number.slice(-8)})`;
            }
            return name;
        }
        
        if (drive.serial_number) {
            return `Drive ${drive.serial_number.slice(-8)}`;
        }
        
        return 'Unknown Drive';
    },

    getDriveType(drive) {
        const isNvme = drive.device?.type?.toLowerCase() === 'nvme' || 
                       drive.device?.protocol === 'NVMe';
        if (isNvme) return 'NVMe';
        
        const isSsd = drive.rotation_rate === 0;
        if (isSsd) return 'SSD';
        
        if (drive.rotation_rate) return `${drive.rotation_rate} RPM`;
        return 'HDD';
    },

    escapeHtml(str) {
        if (!str) return '';
        return str.replace(/'/g, "\\'");
    }
};