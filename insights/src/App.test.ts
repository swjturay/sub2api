import { describe, expect, it } from "vitest";
import { userDataScope } from "./lib/dataScope";
describe("Insights data scope",()=>{it("survives same-user token rotation but changes for account or role",()=>{const admin={id:1,role:"admin"};expect(userDataScope(admin)).toBe(userDataScope({...admin}));expect(userDataScope({id:2,role:"admin"})).not.toBe(userDataScope(admin));expect(userDataScope({id:1,role:"user"})).not.toBe(userDataScope(admin))})});
