export namespace appinfo {
	
	export class Info {
	    version: string;
	    commit: string;
	    platform: string;
	    data_dir: string;
	    log_file: string;
	
	    static createFrom(source: any = {}) {
	        return new Info(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.commit = source["commit"];
	        this.platform = source["platform"];
	        this.data_dir = source["data_dir"];
	        this.log_file = source["log_file"];
	    }
	}

}

export namespace imageproc {
	
	export class Info {
	    path: string;
	    format: string;
	    width: number;
	    height: number;
	    bytes: number;
	    thumbnail: string;
	    warning?: string;
	
	    static createFrom(source: any = {}) {
	        return new Info(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.format = source["format"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.bytes = source["bytes"];
	        this.thumbnail = source["thumbnail"];
	        this.warning = source["warning"];
	    }
	}
	export class Preview {
	    data_url: string;
	    width: number;
	    height: number;
	    has_alpha: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Preview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.data_url = source["data_url"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.has_alpha = source["has_alpha"];
	    }
	}

}

export namespace model {
	
	export class HistoryBatch {
	    id: number;
	    created_at: number;
	    updated_at: number;
	    batch_key: string;
	    output_format: string;
	    target_bytes: number;
	    max_width: number;
	    max_height: number;
	    summary: string;
	    total: number;
	    success: number;
	    failed: number;
	    skipped: number;
	    canceled: number;
	    saved_bytes: number;
	    files_purged: boolean;
	
	    static createFrom(source: any = {}) {
	        return new HistoryBatch(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.created_at = source["created_at"];
	        this.updated_at = source["updated_at"];
	        this.batch_key = source["batch_key"];
	        this.output_format = source["output_format"];
	        this.target_bytes = source["target_bytes"];
	        this.max_width = source["max_width"];
	        this.max_height = source["max_height"];
	        this.summary = source["summary"];
	        this.total = source["total"];
	        this.success = source["success"];
	        this.failed = source["failed"];
	        this.skipped = source["skipped"];
	        this.canceled = source["canceled"];
	        this.saved_bytes = source["saved_bytes"];
	        this.files_purged = source["files_purged"];
	    }
	}
	export class HistoryItem {
	    id: number;
	    created_at: number;
	    updated_at: number;
	    batch_id: number;
	    source_path: string;
	    source_name: string;
	    source_bytes: number;
	    rel_path: string;
	    status: string;
	    message: string;
	    format: string;
	    width: number;
	    height: number;
	    output_bytes: number;
	    thumbnail: string;
	    staging_name: string;
	    downloaded: boolean;
	    downloaded_at: number;
	
	    static createFrom(source: any = {}) {
	        return new HistoryItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.created_at = source["created_at"];
	        this.updated_at = source["updated_at"];
	        this.batch_id = source["batch_id"];
	        this.source_path = source["source_path"];
	        this.source_name = source["source_name"];
	        this.source_bytes = source["source_bytes"];
	        this.rel_path = source["rel_path"];
	        this.status = source["status"];
	        this.message = source["message"];
	        this.format = source["format"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.output_bytes = source["output_bytes"];
	        this.thumbnail = source["thumbnail"];
	        this.staging_name = source["staging_name"];
	        this.downloaded = source["downloaded"];
	        this.downloaded_at = source["downloaded_at"];
	    }
	}
	export class ImagePreset {
	    id: number;
	    created_at: number;
	    updated_at: number;
	    name: string;
	    output_format: string;
	    target_bytes: number;
	    max_width: number;
	    max_height: number;
	
	    static createFrom(source: any = {}) {
	        return new ImagePreset(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.created_at = source["created_at"];
	        this.updated_at = source["updated_at"];
	        this.name = source["name"];
	        this.output_format = source["output_format"];
	        this.target_bytes = source["target_bytes"];
	        this.max_width = source["max_width"];
	        this.max_height = source["max_height"];
	    }
	}

}

export namespace page {
	
	export class Request {
	    page: number;
	    page_size: number;
	
	    static createFrom(source: any = {}) {
	        return new Request(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.page = source["page"];
	        this.page_size = source["page_size"];
	    }
	}
	export class Result_file_processor_internal_model_HistoryBatch_ {
	    list: model.HistoryBatch[];
	    total: number;
	    page: number;
	    page_size: number;
	
	    static createFrom(source: any = {}) {
	        return new Result_file_processor_internal_model_HistoryBatch_(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.list = this.convertValues(source["list"], model.HistoryBatch);
	        this.total = source["total"];
	        this.page = source["page"];
	        this.page_size = source["page_size"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Result_file_processor_internal_model_HistoryItem_ {
	    list: model.HistoryItem[];
	    total: number;
	    page: number;
	    page_size: number;
	
	    static createFrom(source: any = {}) {
	        return new Result_file_processor_internal_model_HistoryItem_(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.list = this.convertValues(source["list"], model.HistoryItem);
	        this.total = source["total"];
	        this.page = source["page"];
	        this.page_size = source["page_size"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Result_file_processor_internal_model_ImagePreset_ {
	    list: model.ImagePreset[];
	    total: number;
	    page: number;
	    page_size: number;
	
	    static createFrom(source: any = {}) {
	        return new Result_file_processor_internal_model_ImagePreset_(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.list = this.convertValues(source["list"], model.ImagePreset);
	        this.total = source["total"];
	        this.page = source["page"];
	        this.page_size = source["page_size"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace service {
	
	export class BatchItem {
	    path: string;
	    rel_path: string;
	
	    static createFrom(source: any = {}) {
	        return new BatchItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.rel_path = source["rel_path"];
	    }
	}
	export class BatchRules {
	    output_format: string;
	    target_bytes: number;
	    max_width: number;
	    max_height: number;
	
	    static createFrom(source: any = {}) {
	        return new BatchRules(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.output_format = source["output_format"];
	        this.target_bytes = source["target_bytes"];
	        this.max_width = source["max_width"];
	        this.max_height = source["max_height"];
	    }
	}
	export class BatchRequest {
	    items: BatchItem[];
	    rules: BatchRules;
	
	    static createFrom(source: any = {}) {
	        return new BatchRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], BatchItem);
	        this.rules = this.convertValues(source["rules"], BatchRules);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ItemResult {
	    path: string;
	    output_path: string;
	    status: string;
	    source_bytes: number;
	    output_bytes: number;
	    format?: string;
	    width: number;
	    height: number;
	    extreme_degraded: boolean;
	    alpha_flattened: boolean;
	    message?: string;
	
	    static createFrom(source: any = {}) {
	        return new ItemResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.output_path = source["output_path"];
	        this.status = source["status"];
	        this.source_bytes = source["source_bytes"];
	        this.output_bytes = source["output_bytes"];
	        this.format = source["format"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.extreme_degraded = source["extreme_degraded"];
	        this.alpha_flattened = source["alpha_flattened"];
	        this.message = source["message"];
	    }
	}
	export class BatchResult {
	    ok: boolean;
	    message?: string;
	    items: ItemResult[];
	    success: number;
	    failed: number;
	    skipped: number;
	    canceled: number;
	    extreme_degraded: number;
	    saved_bytes: number;
	    batch_key: string;
	
	    static createFrom(source: any = {}) {
	        return new BatchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.message = source["message"];
	        this.items = this.convertValues(source["items"], ItemResult);
	        this.success = source["success"];
	        this.failed = source["failed"];
	        this.skipped = source["skipped"];
	        this.canceled = source["canceled"];
	        this.extreme_degraded = source["extreme_degraded"];
	        this.saved_bytes = source["saved_bytes"];
	        this.batch_key = source["batch_key"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class DownloadOptions {
	    directory: string;
	    keep_structure: boolean;
	    pattern: string;
	
	    static createFrom(source: any = {}) {
	        return new DownloadOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.directory = source["directory"];
	        this.keep_structure = source["keep_structure"];
	        this.pattern = source["pattern"];
	    }
	}
	export class DownloadResult {
	    path: string;
	    message?: string;
	    ok: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DownloadResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.message = source["message"];
	        this.ok = source["ok"];
	    }
	}
	
	export class PreviewConvertResult {
	    status: string;
	    message?: string;
	    output_bytes: number;
	    format: string;
	    width: number;
	    height: number;
	    quality: number;
	    extreme_degraded: boolean;
	    alpha_flattened: boolean;
	    preview_url: string;
	
	    static createFrom(source: any = {}) {
	        return new PreviewConvertResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.message = source["message"];
	        this.output_bytes = source["output_bytes"];
	        this.format = source["format"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.quality = source["quality"];
	        this.extreme_degraded = source["extreme_degraded"];
	        this.alpha_flattened = source["alpha_flattened"];
	        this.preview_url = source["preview_url"];
	    }
	}
	export class ScanRejection {
	    path: string;
	    reason: string;
	
	    static createFrom(source: any = {}) {
	        return new ScanRejection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.reason = source["reason"];
	    }
	}
	export class ScannedFile {
	    path: string;
	    name: string;
	    size: number;
	    rel_path: string;
	
	    static createFrom(source: any = {}) {
	        return new ScannedFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.size = source["size"];
	        this.rel_path = source["rel_path"];
	    }
	}
	export class ScanResult {
	    files: ScannedFile[];
	    rejected: ScanRejection[];
	
	    static createFrom(source: any = {}) {
	        return new ScanResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.files = this.convertValues(source["files"], ScannedFile);
	        this.rejected = this.convertValues(source["rejected"], ScanRejection);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

