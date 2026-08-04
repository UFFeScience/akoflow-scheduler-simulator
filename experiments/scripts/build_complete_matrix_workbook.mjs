import fs from "node:fs/promises";
import path from "node:path";
import { Workbook, SpreadsheetFile } from "@oai/artifact-tool";

const repo = "/Users/ovvesley/Workspace/scheduler-simulator";
const matrixRoot = process.argv[2] || "latency-datax-envs-workflows-complete-v1";
const outputDir = path.join(repo, "experiments/results", matrixRoot);
const csvText = await fs.readFile(path.join(outputDir, "combined_results.csv"), "utf8");
const workbook = await Workbook.fromCSV(csvText, {sheetName: "Resultados completos"});
const sheet = workbook.worksheets.getItem("Resultados completos");
sheet.showGridLines = false;
const used = sheet.getUsedRange();
const rowCount = 1765;
const columnCount = 23;
sheet.getRangeByIndexes(0, 0, 1, columnCount).format = {
  fill: "#17365D", font: {bold: true, color: "#FFFFFF"}, wrapText: true,
  rowHeight: 42, verticalAlignment: "center",
};
sheet.getRangeByIndexes(1, 0, rowCount - 1, columnCount).format.borders = {
  insideHorizontal: {style: "thin", color: "#E1E6EB"},
};
sheet.getRange("B2:B1765").format.numberFormat = "0";
sheet.getRange("D2:F1765").format.numberFormat = "0.00";
sheet.getRange("G2:I1765").format.numberFormat = "0.00";
sheet.getRange("J2:K1765").format.numberFormat = "$0.000000";
sheet.getRange("L2:Q1765").format.numberFormat = "#,##0.000";
sheet.getRange("R2:S1765").format.numberFormat = "0.00";
sheet.getRange("A1:W1765").format.autofitColumns();
sheet.getRange("A:A").format.columnWidth = 28;
sheet.getRange("C:C").format.columnWidth = 31;
sheet.getRange("W:W").format.columnWidth = 55;
sheet.freezePanes.freezeRows(1);
sheet.freezePanes.freezeColumns(6);
sheet.tables.add("A1:W1765", true, "CompleteExperimentMatrix").style = "TableStyleMedium2";

const protocol = workbook.worksheets.add("Protocolo");
protocol.showGridLines = false;
protocol.getRange("A1:D1").merge();
protocol.getRange("A1").values = [["Matriz completa: latência, banda, dados e SLA"]];
protocol.getRange("A1:D1").format = {fill:"#17365D",font:{bold:true,color:"#FFFFFF",size:16},rowHeight:28};
protocol.getRange("A3:B12").values = [
  ["Parâmetro","Valores"], ["Latência (ms)","100; 1.000; 10.000"],
  ["Banda (Mbps)","500; 100; 10"], ["Escala dos dados","1×; 10×; 100×"],
  ["Deadline e budget","1,20×; 0,95×; 0,90×; 0,80× do HEFT por ambiente"],
  ["Workflows","7 aplicações WfCommons"], ["Ambientes","7 ambientes"],
  ["Seed","1"], ["Interferência","0"], ["Preço WfCommons","AWS EC2 m6i.12xlarge: US$ 2,304/h"],
];
protocol.getRange("A3:B3").format={fill:"#17365D",font:{bold:true,color:"#FFFFFF"}};
protocol.getRange("A3:B12").format.autofitColumns(); protocol.getRange("A:A").format.columnWidth=28; protocol.getRange("B:B").format.columnWidth=70;

const check = await workbook.inspect({kind:"table",range:"Resultados completos!A1:W10",include:"values,formulas",tableMaxRows:10,tableMaxCols:23});
await fs.writeFile(path.join(outputDir,"workbook-inspection.ndjson"),check.ndjson);
const errors = await workbook.inspect({kind:"match",searchTerm:"#REF!|#DIV/0!|#VALUE!|#NAME\\?|#N/A",options:{useRegex:true,maxResults:100},summary:"formula errors"});
await fs.writeFile(path.join(outputDir,"workbook-errors.ndjson"),errors.ndjson);
for (const [name, range] of [["Resultados completos","A1:W18"],["Protocolo","A1:B12"]]) {
  const rendered = await workbook.render({sheetName:name,range,scale:1.2,format:"png"});
  await fs.writeFile(path.join(outputDir,`preview-${name.replaceAll(" ","-").toLowerCase()}.png`),new Uint8Array(await rendered.arrayBuffer()));
}
const output = await SpreadsheetFile.exportXlsx(workbook);
await output.save(path.join(outputDir,"resultados-completos.xlsx"));
