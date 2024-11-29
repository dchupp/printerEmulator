export namespace main {
	
	export class PrinterDPI {
	    value: number;
	    desc: string;
	
	    static createFrom(source: any = {}) {
	        return new PrinterDPI(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.value = source["value"];
	        this.desc = source["desc"];
	    }
	}
	export class TCPServer {
	
	
	    static createFrom(source: any = {}) {
	        return new TCPServer(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	
	    }
	}

}

