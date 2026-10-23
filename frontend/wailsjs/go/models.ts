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

}

export namespace service {
	
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
	    paths: string[];
	    rules: BatchRules;
	
	    static createFrom(source: any = {}) {
	        return new BatchRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.paths = source["paths"];
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
	    width: number;
	    height: number;
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
	        this.width = source["width"];
	        this.height = source["height"];
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
	    saved_bytes: number;
	
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
	        this.saved_bytes = source["saved_bytes"];
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
	
	    static createFrom(source: any = {}) {
	        return new ScannedFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.size = source["size"];
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

