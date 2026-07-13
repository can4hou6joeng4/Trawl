export namespace app {
	
	export class ExtractResult {
	    success: boolean;
	    recordCount: number;
	    outputPath: string;
	    errorMessage?: string;
	    records?: any[];
	    fieldLabels?: Record<string, string>;
	    ocrPages?: extractor.OcrPageLines[];
	
	    static createFrom(source: any = {}) {
	        return new ExtractResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.recordCount = source["recordCount"];
	        this.outputPath = source["outputPath"];
	        this.errorMessage = source["errorMessage"];
	        this.records = source["records"];
	        this.fieldLabels = source["fieldLabels"];
	        this.ocrPages = this.convertValues(source["ocrPages"], extractor.OcrPageLines);
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
	export class FieldOption {
	    key: string;
	    label: string;
	
	    static createFrom(source: any = {}) {
	        return new FieldOption(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.label = source["label"];
	    }
	}

}

export namespace config {
	
	export class TrialStatus {
	    isActivated: boolean;
	    isExpired: boolean;
	    remaining: number;
	    days: number;
	    hours: number;
	
	    static createFrom(source: any = {}) {
	        return new TrialStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.isActivated = source["isActivated"];
	        this.isExpired = source["isExpired"];
	        this.remaining = source["remaining"];
	        this.days = source["days"];
	        this.hours = source["hours"];
	    }
	}

}

export namespace extractor {
	
	export class OcrLine {
	    text: string;
	    x: number;
	    y: number;
	    w: number;
	    h: number;
	
	    static createFrom(source: any = {}) {
	        return new OcrLine(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.text = source["text"];
	        this.x = source["x"];
	        this.y = source["y"];
	        this.w = source["w"];
	        this.h = source["h"];
	    }
	}
	export class OcrPageLines {
	    page: number;
	    lines: OcrLine[];
	
	    static createFrom(source: any = {}) {
	        return new OcrPageLines(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.page = source["page"];
	        this.lines = this.convertValues(source["lines"], OcrLine);
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

